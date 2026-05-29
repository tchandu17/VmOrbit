package notification

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// Dispatcher evaluates notification rules against a platform event and
// delivers notifications to matching channels. It is called by the
// PlatformEventService after persisting each event.
//
// Delivery is synchronous per-rule but runs in a goroutine so it never
// blocks the event dispatch path.
type Dispatcher struct {
	rules   port.NotificationRuleRepository
	history port.NotificationHistoryRepository
	log     logger.Logger
}

// NewDispatcher creates a new notification Dispatcher.
func NewDispatcher(
	rules port.NotificationRuleRepository,
	history port.NotificationHistoryRepository,
	log logger.Logger,
) *Dispatcher {
	return &Dispatcher{rules: rules, history: history, log: log}
}

// Dispatch evaluates all enabled rules against the event and delivers
// notifications asynchronously. It is safe to call from any goroutine.
func (d *Dispatcher) Dispatch(ctx context.Context, event *model.PlatformEvent) {
	go d.dispatch(event)
}

func (d *Dispatcher) dispatch(event *model.PlatformEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rules, err := d.rules.ListEnabled(ctx)
	if err != nil {
		d.log.Error("notification dispatcher: failed to load rules", logger.Error(err))
		return
	}

	for i := range rules {
		rule := &rules[i]
		if !rule.Matches(event) {
			continue
		}
		if rule.IsThrottled() {
			d.recordHistory(ctx, rule, event, model.NotificationStatusThrottled, "throttled", 0)
			continue
		}
		if rule.Channel == nil {
			d.log.Warn("notification rule has no channel loaded",
				logger.String("rule_id", rule.ID.String()))
			continue
		}
		if !rule.Channel.Enabled {
			continue
		}

		d.deliver(ctx, rule, event)
	}
}

// deliver attempts to send a notification and records the outcome.
func (d *Dispatcher) deliver(ctx context.Context, rule *model.NotificationRule, event *model.PlatformEvent) {
	sender, err := NewSender(rule.Channel)
	if err != nil {
		d.log.Error("notification: unsupported channel type",
			logger.String("channel_type", string(rule.Channel.Type)),
			logger.Error(err))
		d.recordHistory(ctx, rule, event, model.NotificationStatusFailed, err.Error(), 1)
		return
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		lastErr = sender.Send(sendCtx, rule.Channel, event)
		cancel()

		if lastErr == nil {
			break
		}
		d.log.Warn("notification delivery attempt failed",
			logger.String("rule_id", rule.ID.String()),
			logger.String("channel", rule.Channel.Name),
			logger.Int("attempt", attempt),
			logger.Error(lastErr))

		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt*attempt) * time.Second) // 1s, 4s
		}
	}

	if lastErr != nil {
		d.log.Error("notification delivery failed after retries",
			logger.String("rule_id", rule.ID.String()),
			logger.String("channel", rule.Channel.Name),
			logger.Error(lastErr))
		d.recordHistory(ctx, rule, event, model.NotificationStatusFailed, lastErr.Error(), maxAttempts)
		return
	}

	// Success — update throttle timestamp
	if err := d.rules.UpdateLastTriggered(ctx, rule.ID.String()); err != nil {
		d.log.Warn("failed to update last_triggered_at", logger.Error(err))
	}

	d.recordHistory(ctx, rule, event, model.NotificationStatusDelivered, "", maxAttempts)
	d.log.Info("notification delivered",
		logger.String("rule", rule.Name),
		logger.String("channel", rule.Channel.Name),
		logger.String("event_type", string(event.EventType)))
}

func (d *Dispatcher) recordHistory(
	ctx context.Context,
	rule *model.NotificationRule,
	event *model.PlatformEvent,
	status model.NotificationStatus,
	errMsg string,
	attempts int,
) {
	now := time.Now().UTC()
	h := &model.NotificationHistory{
		ID:           uuid.New(),
		CreatedAt:    now,
		RuleID:       rule.ID,
		ChannelID:    rule.ChannelID,
		EventID:      event.ID,
		Status:       status,
		ErrorMessage: errMsg,
		AttemptCount: attempts,
	}
	if status == model.NotificationStatusDelivered {
		h.DeliveredAt = &now
	}
	if err := d.history.Create(ctx, h); err != nil {
		d.log.Warn("failed to record notification history", logger.Error(err))
	}
}
