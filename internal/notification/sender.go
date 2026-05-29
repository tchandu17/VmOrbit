// Package notification provides delivery workers for the notification engine.
// Each sender implements the Sender interface and is responsible for delivering
// a notification to a specific channel type (email, Slack, webhook).
package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Sender interface
// ─────────────────────────────────────────────────────────────────────────────

// Sender delivers a notification for a given event to a channel.
type Sender interface {
	Send(ctx context.Context, channel *model.NotificationChannel, event *model.PlatformEvent) error
}

// NewSender returns the appropriate Sender for the channel type.
func NewSender(ch *model.NotificationChannel) (Sender, error) {
	switch ch.Type {
	case model.NotificationChannelEmail:
		return &emailSender{}, nil
	case model.NotificationChannelSlack:
		return &slackSender{}, nil
	case model.NotificationChannelWebhook:
		return &webhookSender{}, nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Email sender
// ─────────────────────────────────────────────────────────────────────────────

type emailSender struct{}

func (s *emailSender) Send(ctx context.Context, ch *model.NotificationChannel, event *model.PlatformEvent) error {
	cfg := ch.Config

	host, _ := cfg["host"].(string)
	portRaw, _ := cfg["port"].(float64)
	port := int(portRaw)
	if port == 0 {
		port = 587
	}
	username, _ := cfg["username"].(string)
	password, _ := cfg["password"].(string)
	from, _ := cfg["from"].(string)
	if from == "" {
		from = username
	}

	// Recipients
	var to []string
	switch v := cfg["to"].(type) {
	case []interface{}:
		for _, r := range v {
			if addr, ok := r.(string); ok && addr != "" {
				to = append(to, addr)
			}
		}
	case string:
		if v != "" {
			to = append(to, v)
		}
	}
	if len(to) == 0 {
		return fmt.Errorf("email channel %q has no recipients configured", ch.Name)
	}

	subject := fmt.Sprintf("[VMOrbit] %s — %s", strings.ToUpper(string(event.Severity)), event.EventType)
	body := buildEmailBody(event)

	msg := buildMIMEMessage(from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtp.PlainAuth("", username, password, host)

	useTLS, _ := cfg["tls"].(bool)
	if useTLS {
		tlsCfg := &tls.Config{ServerName: host}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		defer conn.Close()
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
		defer client.Close()
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err := client.Mail(from); err != nil {
			return fmt.Errorf("smtp MAIL: %w", err)
		}
		for _, r := range to {
			if err := client.Rcpt(r); err != nil {
				return fmt.Errorf("smtp RCPT %s: %w", r, err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp DATA: %w", err)
		}
		defer w.Close()
		_, err = w.Write([]byte(msg))
		return err
	}

	return smtp.SendMail(addr, auth, from, to, []byte(msg))
}

func buildEmailBody(event *model.PlatformEvent) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Event: %s\n", event.EventType))
	sb.WriteString(fmt.Sprintf("Severity: %s\n", event.Severity))
	sb.WriteString(fmt.Sprintf("Time: %s\n", event.CreatedAt.UTC().Format(time.RFC3339)))
	if event.Provider != "" {
		sb.WriteString(fmt.Sprintf("Provider: %s\n", event.Provider))
	}
	if event.ResourceType != "" {
		sb.WriteString(fmt.Sprintf("Resource: %s\n", event.ResourceType))
	}
	sb.WriteString(fmt.Sprintf("\nMessage:\n%s\n", event.Message))
	if len(event.Metadata) > 0 {
		if b, err := json.MarshalIndent(event.Metadata, "", "  "); err == nil {
			sb.WriteString(fmt.Sprintf("\nMetadata:\n%s\n", string(b)))
		}
	}
	return sb.String()
}

func buildMIMEMessage(from string, to []string, subject, body string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s\r\n", from))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// Slack sender
// ─────────────────────────────────────────────────────────────────────────────

type slackSender struct{}

func (s *slackSender) Send(ctx context.Context, ch *model.NotificationChannel, event *model.PlatformEvent) error {
	cfg := ch.Config

	webhookURL, _ := cfg["webhook_url"].(string)
	if webhookURL == "" {
		return fmt.Errorf("slack channel %q missing webhook_url", ch.Name)
	}

	channel, _ := cfg["channel"].(string)
	username, _ := cfg["username"].(string)
	if username == "" {
		username = "VMOrbit"
	}
	iconEmoji, _ := cfg["icon_emoji"].(string)
	if iconEmoji == "" {
		iconEmoji = ":satellite:"
	}

	color := slackColor(event.Severity)
	text := fmt.Sprintf("*[%s]* %s", strings.ToUpper(string(event.Severity)), event.Message)

	payload := map[string]interface{}{
		"username":   username,
		"icon_emoji": iconEmoji,
		"attachments": []map[string]interface{}{
			{
				"color":    color,
				"title":    fmt.Sprintf("VMOrbit Alert: %s", event.EventType),
				"text":     text,
				"fallback": text,
				"fields":   slackFields(event),
				"ts":       event.CreatedAt.Unix(),
				"footer":   "VMOrbit Notification Engine",
			},
		},
	}
	if channel != "" {
		payload["channel"] = channel
	}

	return postJSON(ctx, webhookURL, nil, payload)
}

func slackColor(severity model.PlatformEventSeverity) string {
	switch severity {
	case model.PlatformEventSeverityCritical:
		return "danger"
	case model.PlatformEventSeverityWarning:
		return "warning"
	default:
		return "good"
	}
}

func slackFields(event *model.PlatformEvent) []map[string]interface{} {
	fields := []map[string]interface{}{}
	if event.Provider != "" {
		fields = append(fields, map[string]interface{}{"title": "Provider", "value": event.Provider, "short": true})
	}
	if event.ResourceType != "" {
		fields = append(fields, map[string]interface{}{"title": "Resource", "value": event.ResourceType, "short": true})
	}
	if event.HypervisorID != nil {
		fields = append(fields, map[string]interface{}{"title": "Hypervisor ID", "value": event.HypervisorID.String(), "short": true})
	}
	return fields
}

// ─────────────────────────────────────────────────────────────────────────────
// Generic webhook sender
// ─────────────────────────────────────────────────────────────────────────────

type webhookSender struct{}

func (s *webhookSender) Send(ctx context.Context, ch *model.NotificationChannel, event *model.PlatformEvent) error {
	cfg := ch.Config

	url, _ := cfg["url"].(string)
	if url == "" {
		return fmt.Errorf("webhook channel %q missing url", ch.Name)
	}

	method, _ := cfg["method"].(string)
	if method == "" {
		method = http.MethodPost
	}

	// Build headers
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "VMOrbit-Notification/1.0",
	}
	if rawHeaders, ok := cfg["headers"].(map[string]interface{}); ok {
		for k, v := range rawHeaders {
			if sv, ok := v.(string); ok {
				headers[k] = sv
			}
		}
	}

	// HMAC secret for signature (optional)
	secret, _ := cfg["secret"].(string)

	payload := map[string]interface{}{
		"event_id":      event.ID.String(),
		"event_type":    event.EventType,
		"severity":      event.Severity,
		"provider":      event.Provider,
		"resource_type": event.ResourceType,
		"message":       event.Message,
		"metadata":      event.Metadata,
		"created_at":    event.CreatedAt.UTC().Format(time.RFC3339),
	}
	if event.ResourceID != nil {
		payload["resource_id"] = event.ResourceID.String()
	}
	if event.HypervisorID != nil {
		payload["hypervisor_id"] = event.HypervisorID.String()
	}

	if secret != "" {
		headers["X-VMOrbit-Secret"] = secret
	}

	return postJSON(ctx, url, headers, payload)
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP helper
// ─────────────────────────────────────────────────────────────────────────────

var httpClient = &http.Client{Timeout: 15 * time.Second}

func postJSON(ctx context.Context, url string, headers map[string]string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
	return nil
}
