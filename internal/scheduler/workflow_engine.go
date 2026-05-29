package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
	"github.com/vmOrbit/backend/pkg/logger"
)

// WorkflowOps bundles the service-layer operations the workflow engine needs.
// Using function types instead of the full service container breaks the
// service → scheduler → service import cycle.
type WorkflowOps struct {
	VMPowerOn      func(ctx context.Context, vmID string) (string, error)
	VMPowerOff     func(ctx context.Context, vmID string) (string, error)
	VMReboot       func(ctx context.Context, vmID string) (string, error)
	VMSnapshot     func(ctx context.Context, vmID string, spec port.SnapshotSpec) (string, error)
	SyncInventory  func(ctx context.Context, hypervisorID string) (string, error)
	ListVMsByTag   func(ctx context.Context, tagID string) ([]string, error)
	DispatchEvent  func(ctx context.Context, req port.EventDispatchRequest) error
}

// WorkflowEngineDeps carries all dependencies for the workflow engine.
type WorkflowEngineDeps struct {
	Workflows    port.WorkflowRepository
	WorkflowRuns port.WorkflowRunRepository
	Ops          WorkflowOps
	EventBus     messaging.EventBus
	Enqueue      EnqueueFunc
	Log          logger.Logger
}

// WorkflowEngine executes automation workflows.
type WorkflowEngine struct {
	deps        WorkflowEngineDeps
	unsubscribe []func()
}

// NewWorkflowEngine creates a new WorkflowEngine.
func NewWorkflowEngine(deps WorkflowEngineDeps) *WorkflowEngine {
	return &WorkflowEngine{deps: deps}
}

// SetOps injects the service operations after wiring (breaks circular dep).
func (e *WorkflowEngine) SetOps(ops WorkflowOps) {
	e.deps.Ops = ops
}

// SetEnqueue injects the task engine enqueue function after wiring.
func (e *WorkflowEngine) SetEnqueue(fn EnqueueFunc) {
	e.deps.Enqueue = fn
}

// Start subscribes to relevant event bus topics.
func (e *WorkflowEngine) Start() {
	bus := e.deps.EventBus

	e.unsubscribe = append(e.unsubscribe,
		bus.Subscribe(messaging.EventHypervisorDisconnected, func(ctx context.Context, ev messaging.Event) {
			e.handleEvent(ctx, model.WorkflowTriggerProviderDisconnected, ev.Payload)
		}),
	)

	e.unsubscribe = append(e.unsubscribe,
		bus.Subscribe(messaging.EventTaskStatusChanged, func(ctx context.Context, ev messaging.Event) {
			payload, ok := ev.Payload.(map[string]interface{})
			if !ok {
				return
			}
			status, _ := payload["status"].(string)
			taskType, _ := payload["type"].(string)
			if status == "failed" && (taskType == "inventory.sync" || taskType == "hypervisor.sync") {
				e.handleEvent(ctx, model.WorkflowTriggerSyncFailure, payload)
			}
			if status == "failed" {
				e.handleEvent(ctx, model.WorkflowTriggerTaskFailure, payload)
			}
		}),
	)

	e.unsubscribe = append(e.unsubscribe,
		bus.Subscribe(messaging.EventVMStatusChanged, func(ctx context.Context, ev messaging.Event) {
			e.handleEvent(ctx, model.WorkflowTriggerVMStateChange, ev.Payload)
		}),
	)

	e.deps.Log.Info("workflow engine started")
}

// Stop unsubscribes from all event bus topics.
func (e *WorkflowEngine) Stop() {
	for _, unsub := range e.unsubscribe {
		unsub()
	}
	e.unsubscribe = nil
	e.deps.Log.Info("workflow engine stopped")
}

// TriggerManual fires a workflow immediately with optional trigger data.
func (e *WorkflowEngine) TriggerManual(ctx context.Context, workflowID string, triggerData model.JSONMap, triggeredBy *uuid.UUID) (string, error) {
	w, err := e.deps.Workflows.GetByID(ctx, workflowID)
	if err != nil {
		return "", fmt.Errorf("workflow not found: %w", err)
	}
	if !w.Enabled {
		return "", fmt.Errorf("workflow %q is disabled", w.Name)
	}

	run, err := e.createRun(ctx, w, model.WorkflowTriggerManual, triggerData, triggeredBy)
	if err != nil {
		return "", err
	}

	go e.executeRun(context.Background(), w, run)
	return run.ID.String(), nil
}

func (e *WorkflowEngine) handleEvent(ctx context.Context, triggerType model.WorkflowTriggerType, payload interface{}) {
	workflows, err := e.deps.Workflows.ListByTrigger(ctx, triggerType)
	if err != nil {
		e.deps.Log.Error("workflow engine: failed to list workflows",
			logger.String("trigger", string(triggerType)),
			logger.Error(err),
		)
		return
	}

	triggerData := model.JSONMap{}
	if m, ok := payload.(map[string]interface{}); ok {
		for k, v := range m {
			triggerData[k] = v
		}
	}

	for i := range workflows {
		w := &workflows[i]
		if !e.checkConditions(w, triggerData) {
			continue
		}

		active, err := e.deps.WorkflowRuns.CountActive(ctx, w.ID.String())
		if err == nil && active >= int64(w.MaxConcurrentRuns) && w.MaxConcurrentRuns > 0 {
			e.deps.Log.Warn("workflow engine: max concurrent runs reached, skipping",
				logger.String("workflow_id", w.ID.String()),
			)
			continue
		}

		run, err := e.createRun(ctx, w, triggerType, triggerData, nil)
		if err != nil {
			e.deps.Log.Error("workflow engine: failed to create run",
				logger.String("workflow_id", w.ID.String()),
				logger.Error(err),
			)
			continue
		}

		go e.executeRun(context.Background(), w, run)
	}
}

func (e *WorkflowEngine) createRun(ctx context.Context, w *model.Workflow, triggerType model.WorkflowTriggerType, triggerData model.JSONMap, triggeredBy *uuid.UUID) (*model.WorkflowRun, error) {
	run := &model.WorkflowRun{
		ID:          uuid.New(),
		CreatedAt:   time.Now().UTC(),
		WorkflowID:  w.ID,
		Status:      model.WorkflowRunStatusPending,
		TriggerType: triggerType,
		TriggerData: triggerData,
		TriggeredBy: triggeredBy,
	}
	if err := e.deps.WorkflowRuns.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("creating workflow run: %w", err)
	}
	return run, nil
}

func (e *WorkflowEngine) executeRun(ctx context.Context, w *model.Workflow, run *model.WorkflowRun) {
	log := e.deps.Log
	now := time.Now().UTC()
	run.Status = model.WorkflowRunStatusRunning
	run.StartedAt = &now
	_ = e.deps.WorkflowRuns.Update(ctx, run)

	logs := []map[string]interface{}{}
	actionsRun := 0
	actionsFailed := 0
	var runErr error

	for i := range w.Actions {
		action := &w.Actions[i]
		actionsRun++

		logEntry := map[string]interface{}{
			"action":      action.Name,
			"action_type": string(action.ActionType),
			"order":       action.Order,
			"started_at":  time.Now().UTC().Format(time.RFC3339),
		}

		err := e.executeAction(ctx, w, run, action)
		logEntry["completed_at"] = time.Now().UTC().Format(time.RFC3339)

		if err != nil {
			actionsFailed++
			logEntry["error"] = err.Error()
			logEntry["status"] = "failed"
			log.Warn("workflow engine: action failed",
				logger.String("workflow_id", w.ID.String()),
				logger.String("action", action.Name),
				logger.Error(err),
			)

			continueOnErr := w.ContinueOnError
			if action.ContinueOnError != nil {
				continueOnErr = *action.ContinueOnError
			}
			if !continueOnErr {
				runErr = err
				logs = append(logs, logEntry)
				break
			}
		} else {
			logEntry["status"] = "completed"
		}

		logs = append(logs, logEntry)
	}

	completedAt := time.Now().UTC()
	run.CompletedAt = &completedAt
	run.ActionsRun = actionsRun
	run.ActionsFailed = actionsFailed
	run.Logs = model.JSONMap{"entries": logs}

	if runErr != nil {
		run.Status = model.WorkflowRunStatusFailed
		run.ErrorMessage = runErr.Error()
	} else {
		run.Status = model.WorkflowRunStatusCompleted
	}

	_ = e.deps.WorkflowRuns.Update(ctx, run)

	newRunCount := w.RunCount + 1
	newFailureCount := w.FailureCount
	lastRunStatus := "success"
	if runErr != nil {
		newFailureCount++
		lastRunStatus = "failed"
	}
	_ = e.deps.Workflows.UpdateAfterRun(ctx, w.ID.String(), port.WorkflowRunUpdate{
		LastRunAt:     completedAt,
		LastRunStatus: lastRunStatus,
		RunCount:      newRunCount,
		FailureCount:  newFailureCount,
	})

	log.Info("workflow run completed",
		logger.String("workflow_id", w.ID.String()),
		logger.String("run_id", run.ID.String()),
		logger.String("status", string(run.Status)),
	)
}

func (e *WorkflowEngine) executeAction(ctx context.Context, w *model.Workflow, run *model.WorkflowRun, action *model.WorkflowAction) error {
	timeout := time.Duration(action.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	actionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	maxAttempts := action.RetryCount + 1
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = e.runAction(actionCtx, w, run, action)
		if lastErr == nil {
			return nil
		}
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
		}
	}
	return lastErr
}

func (e *WorkflowEngine) runAction(ctx context.Context, w *model.Workflow, run *model.WorkflowRun, action *model.WorkflowAction) error {
	cfg := action.Config
	if cfg == nil {
		cfg = model.JSONMap{}
	}
	ops := e.deps.Ops

	switch action.ActionType {
	case model.WorkflowActionCreateSnapshot:
		return e.actionCreateSnapshot(ctx, cfg, ops)

	case model.WorkflowActionPowerOn:
		return e.actionPowerOp(ctx, cfg, ops, ops.VMPowerOn)

	case model.WorkflowActionPowerOff:
		return e.actionPowerOp(ctx, cfg, ops, ops.VMPowerOff)

	case model.WorkflowActionReboot:
		return e.actionPowerOp(ctx, cfg, ops, ops.VMReboot)

	case model.WorkflowActionSendNotification:
		return e.actionSendNotification(ctx, cfg, w, run, ops)

	case model.WorkflowActionTriggerSync:
		return e.actionTriggerSync(ctx, cfg, ops)

	case model.WorkflowActionWebhook:
		return e.actionWebhook(ctx, cfg, run)

	case model.WorkflowActionDelay:
		seconds := 0
		if v, ok := cfg["seconds"].(float64); ok {
			seconds = int(v)
		}
		if seconds > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(seconds) * time.Second):
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported action type: %s", action.ActionType)
	}
}

func (e *WorkflowEngine) actionCreateSnapshot(ctx context.Context, cfg model.JSONMap, ops WorkflowOps) error {
	targetType, _ := cfg["target_type"].(string)
	targetIDs := extractStringSlice(cfg, "target_ids")
	nameTemplate, _ := cfg["name_template"].(string)
	if nameTemplate == "" {
		nameTemplate = fmt.Sprintf("workflow-%s", time.Now().UTC().Format("2006-01-02T15-04"))
	}
	description, _ := cfg["description"].(string)

	vmIDs := targetIDs
	if targetType == "tag" && ops.ListVMsByTag != nil {
		vmIDs = nil
		for _, tagID := range targetIDs {
			ids, err := ops.ListVMsByTag(ctx, tagID)
			if err != nil {
				return fmt.Errorf("resolving tag %s: %w", tagID, err)
			}
			vmIDs = append(vmIDs, ids...)
		}
	}

	for _, vmID := range vmIDs {
		taskID, err := ops.VMSnapshot(ctx, vmID, port.SnapshotSpec{
			Name:        nameTemplate,
			Description: description,
		})
		if err != nil {
			return fmt.Errorf("snapshot VM %s: %w", vmID, err)
		}
		if e.deps.Enqueue != nil {
			_ = e.deps.Enqueue(ctx, taskID, 5)
		}
	}
	return nil
}

func (e *WorkflowEngine) actionPowerOp(ctx context.Context, cfg model.JSONMap, ops WorkflowOps, op func(context.Context, string) (string, error)) error {
	targetType, _ := cfg["target_type"].(string)
	targetIDs := extractStringSlice(cfg, "target_ids")

	vmIDs := targetIDs
	if targetType == "tag" && ops.ListVMsByTag != nil {
		vmIDs = nil
		for _, tagID := range targetIDs {
			ids, err := ops.ListVMsByTag(ctx, tagID)
			if err != nil {
				return fmt.Errorf("resolving tag %s: %w", tagID, err)
			}
			vmIDs = append(vmIDs, ids...)
		}
	}

	for _, vmID := range vmIDs {
		taskID, err := op(ctx, vmID)
		if err != nil {
			return fmt.Errorf("power op on VM %s: %w", vmID, err)
		}
		if e.deps.Enqueue != nil {
			_ = e.deps.Enqueue(ctx, taskID, 5)
		}
	}
	return nil
}

func (e *WorkflowEngine) actionSendNotification(ctx context.Context, cfg model.JSONMap, w *model.Workflow, run *model.WorkflowRun, ops WorkflowOps) error {
	if ops.DispatchEvent == nil {
		return nil
	}
	message, _ := cfg["message"].(string)
	if message == "" {
		message = fmt.Sprintf("Workflow '%s' executed (run %s)", w.Name, run.ID.String())
	}
	return ops.DispatchEvent(ctx, port.EventDispatchRequest{
		EventType:    model.PlatformEventWorkflowExecuted,
		Severity:     model.PlatformEventSeverityInfo,
		ResourceType: "workflow",
		ResourceID:   w.ID.String(),
		Message:      message,
		Metadata: model.JSONMap{
			"workflow_id":   w.ID.String(),
			"workflow_name": w.Name,
			"run_id":        run.ID.String(),
		},
	})
}

func (e *WorkflowEngine) actionTriggerSync(ctx context.Context, cfg model.JSONMap, ops WorkflowOps) error {
	hypervisorID, _ := cfg["hypervisor_id"].(string)
	if hypervisorID == "" {
		return fmt.Errorf("trigger_sync action missing hypervisor_id")
	}
	taskID, err := ops.SyncInventory(ctx, hypervisorID)
	if err != nil {
		return fmt.Errorf("trigger sync for hypervisor %s: %w", hypervisorID, err)
	}
	if e.deps.Enqueue != nil {
		return e.deps.Enqueue(ctx, taskID, 5)
	}
	return nil
}

func (e *WorkflowEngine) actionWebhook(ctx context.Context, cfg model.JSONMap, run *model.WorkflowRun) error {
	url, _ := cfg["url"].(string)
	if url == "" {
		return fmt.Errorf("webhook action missing url")
	}
	method, _ := cfg["method"].(string)
	if method == "" {
		method = http.MethodPost
	}

	body := map[string]interface{}{
		"run_id":      run.ID.String(),
		"workflow_id": run.WorkflowID.String(),
		"trigger":     string(run.TriggerType),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	if customBody, ok := cfg["body"].(string); ok && customBody != "" {
		body["message"] = customBody
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal webhook body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "VMOrbit-Workflow/1.0")

	if headers, ok := cfg["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if sv, ok := v.(string); ok {
				req.Header.Set(k, sv)
			}
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (e *WorkflowEngine) checkConditions(w *model.Workflow, triggerData model.JSONMap) bool {
	if w.Conditions == nil || len(w.Conditions) == 0 {
		return true
	}
	rules, ok := w.Conditions["rules"].([]interface{})
	if !ok {
		return true
	}
	for _, r := range rules {
		rule, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		field, _ := rule["field"].(string)
		operator, _ := rule["operator"].(string)
		value := rule["value"]
		actual := triggerData[field]
		if !evaluateCondition(actual, operator, value) {
			return false
		}
	}
	return true
}

func evaluateCondition(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "eq", "equals":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "neq", "not_equals":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	case "exists":
		return actual != nil
	default:
		return true
	}
}

func extractStringSlice(cfg model.JSONMap, key string) []string {
	raw, ok := cfg[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
