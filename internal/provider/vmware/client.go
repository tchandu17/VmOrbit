package vmware

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"time"

	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

// connectionManager wraps a govmomi.Client and provides session lifecycle
// helpers. It is created by Connect and destroyed by Disconnect.
//
// Thread-safety: the underlying govmomi.Client is safe for concurrent use.
// The connectionManager itself is replaced atomically by the Provider (under
// BaseProvider's mutex) so callers should not cache the pointer.
type connectionManager struct {
	client *govmomi.Client
}

// newConnectionManager dials vCenter/ESXi, authenticates, and returns a ready
// connectionManager. The caller is responsible for calling logout when done.
//
// When TLSVerify is false we also relax the minimum TLS version to 1.0 so
// that older ESXi hosts (5.x, 6.0, 6.5) which only support TLS 1.0/1.1
// can still connect. Go's default minimum is TLS 1.2 which rejects them.
func newConnectionManager(ctx context.Context, creds port.Credentials, timeout time.Duration) (*connectionManager, error) {
	vcURL, err := buildVCenterURL(creds)
	if err != nil {
		return nil, fmt.Errorf("invalid vCenter URL: %w", err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var client *govmomi.Client

	if !creds.TLSVerify {
		// Build the SOAP client manually with a permissive TLS config.
		// Old ESXi (5.x / 6.0 / 6.5) requires:
		//   - TLS 1.0 minimum (Go 1.18+ defaults to TLS 1.2)
		//   - RSA key exchange ciphers (Go 1.22 removed them from the default list)
		//   - InsecureSkipVerify for self-signed certificates
		legacyTLS := &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // intentional for lab/self-signed use
			MinVersion:         tls.VersionTLS10,
			MaxVersion:         tls.VersionTLS13,
			// Explicitly include legacy RSA key exchange ciphers removed in Go 1.22.
			// Required for ESXi 5.x / 6.0 / 6.5 which only offer these suites.
			CipherSuites: []uint16{
				// TLS 1.3 (handled automatically, listed for clarity)
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				// TLS 1.2 modern
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
				// TLS 1.2 / 1.1 / 1.0 legacy RSA key exchange (needed for old ESXi)
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, //nolint:gosec // needed for very old ESXi
			},
		}
		soapClient := soap.NewClient(vcURL, true)
		soapClient.DefaultTransport().TLSClientConfig = legacyTLS
		vimClient, err := vim25.NewClient(dialCtx, soapClient)
		if err != nil {
			return nil, fmt.Errorf("govmomi dial: %w", err)
		}
		client = &govmomi.Client{
			Client:         vimClient,
			SessionManager: session.NewManager(vimClient),
		}
		if vcURL.User != nil {
			if err := client.Login(dialCtx, vcURL.User); err != nil {
				return nil, fmt.Errorf("govmomi login: %w", err)
			}
		}
	} else {
		// Standard path — Go's default TLS 1.2+ with cert verification.
		client, err = govmomi.NewClient(dialCtx, vcURL, false)
		if err != nil {
			return nil, fmt.Errorf("govmomi dial: %w", err)
		}
	}

	return &connectionManager{client: client}, nil
}

// logout ends the vCenter session and releases the underlying HTTP connection.
func (c *connectionManager) logout(ctx context.Context) error {
	return c.client.Logout(ctx)
}

// ping checks whether the current session is still active.
// It uses SessionManager.SessionIsActive which is a lightweight RPC.
func (c *connectionManager) ping(ctx context.Context) error {
	sm := session.NewManager(c.client.Client)
	active, err := sm.SessionIsActive(ctx)
	if err != nil {
		return fmt.Errorf("session check RPC failed: %w", err)
	}
	if !active {
		return fmt.Errorf("vCenter session is no longer active")
	}
	return nil
}

// serviceVersion returns the vCenter API version string for logging.
func (c *connectionManager) serviceVersion() string {
	if c.client == nil || c.client.Client == nil {
		return "unknown"
	}
	return c.client.Client.ServiceContent.About.ApiVersion
}

// buildVCenterURL constructs the SDK endpoint URL from credentials.
// Format: https://<user>:<password>@<host>:<port>/sdk
func buildVCenterURL(creds port.Credentials) (*url.URL, error) {
	port_ := creds.Port
	if port_ == 0 {
		port_ = 443
	}

	raw := fmt.Sprintf("https://%s:%d/sdk", creds.Host, port_)
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(creds.Username, creds.Password)

	// govmomi also accepts a token via the extra map (e.g. for SSO tokens).
	if creds.Token != "" {
		// Pass the token as a query parameter understood by govmomi's SAML flow.
		q := u.Query()
		q.Set("token", creds.Token)
		u.RawQuery = q.Encode()
	}

	return u, nil
}

// keepAliveInterval is how often the session keep-alive heartbeat fires.
// vCenter sessions expire after 30 minutes of inactivity by default.
const keepAliveInterval = 10 * time.Minute

// startKeepAlive launches a background goroutine that periodically calls
// SessionManager.SessionIsActive to prevent the vCenter session from expiring.
// Cancel ctx to stop the goroutine.
//
// This is optional — call it after a successful Connect if you expect long
// idle periods between operations.
func (c *connectionManager) startKeepAlive(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(keepAliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sm := session.NewManager(c.client.Client)
				if _, err := sm.SessionIsActive(ctx); err != nil {
					// Session may have expired; the health-check loop in Manager
					// will detect this via Ping and trigger a reconnect.
					return
				}
			}
		}
	}()
}
