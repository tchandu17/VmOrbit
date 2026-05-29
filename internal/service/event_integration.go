package service

import (
	"context"
	"fmt"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
	"github.com/vmOrbit/backend/pkg/logger"
)

// EventIntegration subscribes to the internal messaging.EventBus and
// translates infrastructure events into PlatformEvents that are persisted
// and trigger notification delivery.
//
// This keeps the task engine and health engine decoupled from the notification
// system — they publish to the event bus as before, and this subscriber
// bridges the gap.
type EventIntegration struct {
	eventBus    messaging.EventBus
	platformSvc port.PlatformEventService
	log         logger.Logger
	unsubs      []func()
}

// NewEventIntegration creates a new EventIntegration.
func NewEventIntegration(
	eventBus messaging.EventBus,
	platformSvc port.PlatformEventService,
	log logger.Logger,
) *EventIntegration {
	return &EventIntegration{
		eventBus:    eventBus,
		platformSvc: platformSvc,
		log:         log,
	}
}

// Start registers all event bus subscriptions. Call once during bootstrap.
func (i *EventIntegration) Start() {
	i.unsubs = append(i.unsubs,
		i.eventBus.Subscribe(messaging.EventHypervisorConnected, i.onHypervisorConnected),
		i.eventBus.Subscribe(messaging.EventHypervisorDisconnected, i.onHypervisorDisconnected),
		i.eventBus.Subscribe(messaging.EventInventorySynced, i.onInventorySynced),
		i.eventBus.Subscribe(messaging.EventTaskStatusChanged, i.onTaskStatusChanged),
	)
	i.log.Info("event integration: subscriptions registered")
}

// Stop unregisters all subscriptions.
func (i *EventIntegration) Stop() {
	for _, unsub := range i.unsubs {
		unsub()
	}
	i.unsubs = nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

func (i *EventIntegration) onHypervisorConnected(ctx context.Context, e messaging.Event) {
	payload := asMap(e.Payload)
	hypervisorID, _ := payload["hypervisor_id"].(string)
	provider, _ := payload["provider"].(string)
	name, _ := payload["name"].(string)

	msg := fmt.Sprintf("Provider connected: %s", name)
	if name == "" {
		msg = "Provider connected"
	}

	_ = i.platformSvc.Dispatch(ctx, port.EventDispatchRequest{
		EventType:    model.PlatformEventProviderConnected,
		Provider:     provider,
		ResourceType: "hypervisor",
		ResourceID:   hypervisorID,
		HypervisorID: hypervisorID,
		Message:      msg,
		Metadata:     model.JSONMap{"name": name, "provider": provider},
	})
}

func (i *EventIntegration) onHypervisorDisconnected(ctx context.Context, e messaging.Event) {
	payload := asMap(e.Payload)
	hypervisorID, _ := payload["hypervisor_id"].(string)
	provider, _ := payload["provider"].(string)
	name, _ := payload["name"].(string)
	reason, _ := payload["reason"].(string)

	msg := fmt.Sprintf("Provider disconnected: %s", name)
	if name == "" {
		msg = "Provider disconnected"
	}
	if reason != "" {
		msg += " — " + reason
	}

	_ = i.platformSvc.Dispatch(ctx, port.EventDispatchRequest{
		EventType:    model.PlatformEventProviderDisconnected,
		Provider:     provider,
		ResourceType: "hypervisor",
		ResourceID:   hypervisorID,
		HypervisorID: hypervisorID,
		Message:      msg,
		Metadata:     model.JSONMap{"name": name, "provider": provider, "reason": reason},
	})
}

func (i *EventIntegration) onInventorySynced(ctx context.Context, e messaging.Event) {
	payload := asMap(e.Payload)
	hypervisorID, _ := payload["hypervisor_id"].(string)
	provider, _ := payload["provider"].(string)
	taskID, _ := payload["task_id"].(string)
	success, _ := payload["success"].(bool)
	errMsg, _ := payload["error"].(string)

	if success || errMsg == "" {
		_ = i.platformSvc.Dispatch(ctx, port.EventDispatchRequest{
			EventType:    model.PlatformEventSyncCompleted,
			Provider:     provider,
			ResourceType: "hypervisor",
			HypervisorID: hypervisorID,
			Message:      "Inventory sync completed successfully",
			Metadata:     model.JSONMap{"task_id": taskID, "hypervisor_id": hypervisorID},
		})
	} else {
		_ = i.platformSvc.Dispatch(ctx, port.EventDispatchRequest{
			EventType:    model.PlatformEventSyncFailed,
			Provider:     provider,
			ResourceType: "hypervisor",
			HypervisorID: hypervisorID,
			Message:      fmt.Sprintf("Inventory sync failed: %s", errMsg),
			Metadata:     model.JSONMap{"task_id": taskID, "hypervisor_id": hypervisorID, "error": errMsg},
		})
	}
}

func (i *EventIntegration) onTaskStatusChanged(ctx context.Context, e messaging.Event) {
	payload := asMap(e.Payload)
	status, _ := payload["status"].(string)
	if status != string(model.TaskStatusFailed) {
		return // only care about failures here
	}

	taskID, _ := payload["task_id"].(string)
	taskType, _ := payload["task_type"].(string)
	hypervisorID, _ := payload["hypervisor_id"].(string)
	vmID, _ := payload["vm_id"].(string)
	errMsg, _ := payload["error_message"].(string)

	// Determine the specific platform event type based on task type
	eventType, platformMsg := taskFailureEventType(model.TaskType(taskType), errMsg)

	metadata := model.JSONMap{
		"task_id":   taskID,
		"task_type": taskType,
	}
	if errMsg != "" {
		metadata["error"] = errMsg
	}
	if vmID != "" {
		metadata["vm_id"] = vmID
	}

	_ = i.platformSvc.Dispatch(ctx, port.EventDispatchRequest{
		EventType:    eventType,
		ResourceType: resourceTypeForTask(model.TaskType(taskType)),
		ResourceID:   vmID,
		HypervisorID: hypervisorID,
		Message:      platformMsg,
		Metadata:     metadata,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func asMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func taskFailureEventType(taskType model.TaskType, errMsg string) (model.PlatformEventType, string) {
	suffix := ""
	if errMsg != "" {
		suffix = ": " + errMsg
	}
	switch taskType {
	case model.TaskTypeVMPowerOn:
		return model.PlatformEventVMPowerOnFailed, "VM power-on failed" + suffix
	case model.TaskTypeVMPowerOff:
		return model.PlatformEventVMPowerOffFailed, "VM power-off failed" + suffix
	case model.TaskTypeVMReboot:
		return model.PlatformEventVMRebootFailed, "VM reboot failed" + suffix
	case model.TaskTypeVMSnapshot:
		return model.PlatformEventSnapshotFailed, "Snapshot creation failed" + suffix
	case model.TaskTypeInventorySync, model.TaskTypeHypervisorSync:
		return model.PlatformEventSyncFailed, "Inventory sync failed" + suffix
	case model.TaskTypeVMBulkPowerOn, model.TaskTypeVMBulkPowerOff,
		model.TaskTypeVMBulkReboot, model.TaskTypeVMBulkSnapshot:
		return model.PlatformEventBulkOperationFailed, "Bulk operation failed" + suffix
	default:
		return model.PlatformEventTaskFailed, fmt.Sprintf("Task %s failed%s", taskType, suffix)
	}
}

func resourceTypeForTask(taskType model.TaskType) string {
	switch taskType {
	case model.TaskTypeInventorySync, model.TaskTypeHypervisorSync:
		return "hypervisor"
	case model.TaskTypeVMSnapshot, model.TaskTypeVMSnapshotDelete, model.TaskTypeVMRestore:
		return "snapshot"
	default:
		return "vm"
	}
}
