package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/api/middleware"
	"github.com/vmOrbit/backend/internal/crypto"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/pkg/logger"
)

type hypervisorService struct {
	repo         port.HypervisorRepository
	tasks        port.TaskRepository
	vms          port.VMRepository
	datastores   port.DataStoreRepository
	networks     port.NetworkRepository
	clusters     port.ClusterRepository
	hosts        port.HostRepository
	registry     *provider.Registry
	eventBus     messaging.EventBus
	audit        port.AuditService
	healthRepo   port.ProviderHealthRepository
	log          logger.Logger
}

// NewHypervisorService creates a new hypervisor service.
func NewHypervisorService(
	repo port.HypervisorRepository,
	registry *provider.Registry,
	tasks port.TaskRepository,
	vms port.VMRepository,
	datastores port.DataStoreRepository,
	networks port.NetworkRepository,
	clusters port.ClusterRepository,
	hosts port.HostRepository,
	eventBus messaging.EventBus,
	audit port.AuditService,
	healthRepo port.ProviderHealthRepository,
	log logger.Logger,
) port.HypervisorService {
	return &hypervisorService{
		repo:       repo,
		registry:   registry,
		tasks:      tasks,
		vms:        vms,
		datastores: datastores,
		networks:   networks,
		clusters:   clusters,
		hosts:      hosts,
		eventBus:   eventBus,
		audit:      audit,
		healthRepo: healthRepo,
		log:        log,
	}
}

func (s *hypervisorService) Register(ctx context.Context, req port.RegisterHypervisorRequest) (*model.Hypervisor, error) {
	// Validate provider is registered
	if _, err := s.registry.Get(req.Provider); err != nil {
		return nil, fmt.Errorf("unsupported provider %q: %w", req.Provider, err)
	}

	// Determine the secret to encrypt.
	// For Proxmox: Password holds the API token UUID secret; Token holds the token ID (stored in metadata).
	// For VMware/others: prefer Token over Password if both are set.
	secret := req.Password
	if req.Token != "" && req.Password == "" {
		// Non-Proxmox path: only a token was provided, no separate password.
		secret = req.Token
	}
	// For Proxmox the handler always sets Password = UUID secret and Token = token ID,
	// so secret = req.Password is already correct above.

	// Encrypt the secret before storing
	encrypted, err := encryptSecret(secret)
	if err != nil {
		return nil, fmt.Errorf("encrypting secret: %w", err)
	}

	h := &model.Hypervisor{
		Name:             req.Name,
		Description:      req.Description,
		Provider:         req.Provider,
		Host:             req.Host,
		Port:             req.Port,
		Username:         req.Username,
		EncryptedSecret:  encrypted,
		TLSVerify:        req.TLSVerify,
		Tags:             req.Tags,
		Metadata:         req.Metadata,
		ConnectionStatus: model.ConnectionStatusUnknown,
	}

	if err := s.repo.Create(ctx, h); err != nil {
		return nil, fmt.Errorf("creating hypervisor: %w", err)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionCreate,
		Resource:   "hypervisor",
		ResourceID: h.ID.String(),
		Success:    true,
	})

	return h, nil
}

func (s *hypervisorService) GetByID(ctx context.Context, id string) (*model.Hypervisor, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *hypervisorService) Update(ctx context.Context, id string, req port.UpdateHypervisorRequest) (*model.Hypervisor, error) {
	h, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		h.Name = *req.Name
	}
	if req.Description != nil {
		h.Description = *req.Description
	}
	if req.Host != nil {
		h.Host = *req.Host
	}
	if req.Port != nil {
		h.Port = *req.Port
	}
	if req.Username != nil {
		h.Username = *req.Username
	}
	if req.TLSVerify != nil {
		h.TLSVerify = *req.TLSVerify
	}
	if req.Tags != nil {
		h.Tags = req.Tags
	}

	// Update credential if provided.
	// For Proxmox: Password = UUID secret, Token = token ID (metadata).
	// For others: Token takes precedence over Password.
	secret := ""
	if req.Password != nil && *req.Password != "" {
		secret = *req.Password
	}
	if req.Token != nil && *req.Token != "" && secret == "" {
		// Only use Token as secret when no Password was provided (non-Proxmox path).
		secret = *req.Token
	}
	if secret != "" {
		encrypted, err := encryptSecret(secret)
		if err != nil {
			return nil, fmt.Errorf("encrypting secret: %w", err)
		}
		h.EncryptedSecret = encrypted
	}

	// Merge metadata
	if req.Metadata != nil {
		if h.Metadata == nil {
			h.Metadata = model.JSONMap{}
		}
		for k, v := range req.Metadata {
			h.Metadata[k] = v
		}
	}

	if err := s.repo.Update(ctx, h); err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionUpdate,
		Resource:   "hypervisor",
		ResourceID: id,
		Success:    true,
	})

	return h, nil
}

func (s *hypervisorService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionDelete,
		Resource:   "hypervisor",
		ResourceID: id,
		Success:    true,
	})
	return nil
}

func (s *hypervisorService) List(ctx context.Context, page port.Page) (*port.PageResult[model.Hypervisor], error) {
	return s.repo.List(ctx, page)
}

func (s *hypervisorService) TestConnection(ctx context.Context, id string) error {
	h, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Get the provider type from the registry to validate it's supported,
	// but use a fresh instance via SyncInventoryNow's connect logic to avoid
	// sharing state with the singleton registry provider.
	if _, err := s.registry.Get(h.Provider); err != nil {
		return fmt.Errorf("unsupported provider %q: %w", h.Provider, err)
	}

	creds, err := s.buildCredentials(h)
	if err != nil {
		return err
	}

	// Get a fresh provider instance — don't use the shared registry singleton
	// which may have stale session state from a previous connect/disconnect.
	p, err := s.registry.Get(h.Provider)
	if err != nil {
		return err
	}

	if err := p.Connect(ctx, creds); err != nil {
		_ = s.repo.UpdateConnectionStatus(ctx, id, model.ConnectionStatusError)
		if isTLSError(err) {
			return fmt.Errorf("TLS certificate verification failed — set tls_verify=false for self-signed certificates: %w", err)
		}
		return fmt.Errorf("connection failed: %w", err)
	}
	// Disconnect after test — don't leave a dangling session on the singleton.
	defer p.Disconnect(ctx) //nolint:errcheck

	_ = s.repo.UpdateConnectionStatus(ctx, id, model.ConnectionStatusConnected)
	return nil
}

// BuildCredentials decrypts and returns the connection credentials for a hypervisor.
// This is used by the task engine to connect to the provider for VM operations.
func (s *hypervisorService) BuildCredentials(ctx context.Context, id string) (port.Credentials, model.ProviderType, error) {
	h, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return port.Credentials{}, "", fmt.Errorf("hypervisor not found: %w", err)
	}
	creds, err := s.buildCredentials(h)
	if err != nil {
		return port.Credentials{}, "", err
	}
	return creds, h.Provider, nil
}

// SyncInventory creates an async task and returns its ID (HTTP 202 path).
func (s *hypervisorService) SyncInventory(ctx context.Context, id string) (string, error) {
	// Verify hypervisor exists
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return "", err
	}

	taskID := uuid.New()
	now := time.Now().UTC()
	hypervisorUUID, _ := uuid.Parse(id)
	t := &model.Task{
		Type:         model.TaskTypeInventorySync,
		Status:       model.TaskStatusPending,
		Priority:     5,
		MaxRetries:   3,
		HypervisorID: &hypervisorUUID,
		ScheduledAt:  &now,
		Payload:      model.JSONMap{"hypervisor_id": id},
		CreatedBy:    callerUUID(ctx),
	}
	t.ID = taskID

	if err := s.tasks.Create(ctx, t); err != nil {
		return "", fmt.Errorf("creating sync task: %w", err)
	}

	return taskID.String(), nil
}

// SyncInventoryNow performs the full inventory synchronisation synchronously.
// It is called by the task engine worker — not directly by HTTP handlers.
// progress is a callback for reporting incremental progress (0–100).
func (s *hypervisorService) SyncInventoryNow(ctx context.Context, id string, progress func(pct int, msg string)) (*port.SyncResult, error) {
	if progress == nil {
		progress = func(int, string) {}
	}

	// ── 1. Load hypervisor ────────────────────────────────────────────────────
	h, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("hypervisor not found: %w", err)
	}

	progress(5, "connecting to hypervisor")

	// ── 2. Build credentials & connect ───────────────────────────────────────
	creds, err := s.buildCredentials(h)
	if err != nil {
		return nil, fmt.Errorf("building credentials: %w", err)
	}

	p, err := s.registry.Get(h.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider not registered: %w", err)
	}

	if err := p.Connect(ctx, creds); err != nil {
		_ = s.repo.UpdateConnectionStatus(ctx, id, model.ConnectionStatusError)
		// Surface a clear hint for the most common self-signed cert failure.
		if isTLSError(err) {
			return nil, fmt.Errorf("TLS certificate verification failed — set tls_verify=false for self-signed certificates: %w", err)
		}
		return nil, fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(ctx) //nolint:errcheck

	_ = s.repo.UpdateConnectionStatus(ctx, id, model.ConnectionStatusConnected)
	progress(15, "fetching inventory from provider")

	// ── 3. Fetch live inventory ───────────────────────────────────────────────
	snap, err := p.SyncInventory(ctx)
	if err != nil {
		_ = s.repo.UpdateConnectionStatus(ctx, id, model.ConnectionStatusError)
		return nil, fmt.Errorf("provider SyncInventory: %w", err)
	}

	progress(40, fmt.Sprintf("discovered %d VMs, %d datastores, %d networks",
		len(snap.VMs), len(snap.DataStores), len(snap.Networks)))

	result := &port.SyncResult{HypervisorID: id}
	hypervisorUUID, _ := uuid.Parse(id)

	// ── 4. Upsert VMs ─────────────────────────────────────────────────────────
	if len(snap.VMs) > 0 {
		vmModels := make([]model.VM, 0, len(snap.VMs))
		activeProviderIDs := make([]string, 0, len(snap.VMs))

		for _, info := range snap.VMs {
			activeProviderIDs = append(activeProviderIDs, info.ProviderVMID)
			vmModels = append(vmModels, mapVMInfoToModel(info, hypervisorUUID))
		}

		if err := s.vms.BulkUpsert(ctx, vmModels); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("vm upsert: %v", err))
			s.log.Error("inventory sync: VM upsert failed",
				logger.String("hypervisor_id", id),
				logger.Error(err),
			)
		} else {
			result.VMsUpdated = len(vmModels)
		}

		// Incremental sync: soft-delete VMs no longer reported by the provider.
		removed, err := s.vms.MarkDeletedByProviderIDs(ctx, id, activeProviderIDs)
		if err != nil {
			s.log.Warn("inventory sync: failed to mark deleted VMs",
				logger.String("hypervisor_id", id),
				logger.Error(err),
			)
		} else {
			result.VMsRemoved = int(removed)
		}
	}

	progress(65, "persisting datastores")

	// ── 5. Upsert DataStores ──────────────────────────────────────────────────
	if len(snap.DataStores) > 0 {
		dsModels := make([]model.DataStore, 0, len(snap.DataStores))
		for _, ds := range snap.DataStores {
			dsModels = append(dsModels, model.DataStore{
				HypervisorID: id,
				ProviderID:   ds.ProviderID,
				Name:         ds.Name,
				Type:         ds.Type,
				CapacityGB:   ds.CapacityGB,
				UsedGB:       ds.UsedGB,
				FreeGB:       ds.FreeGB,
				Accessible:   ds.Accessible,
			})
		}
		if err := s.datastores.BulkUpsert(ctx, dsModels); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("datastore upsert: %v", err))
			s.log.Error("inventory sync: datastore upsert failed",
				logger.String("hypervisor_id", id),
				logger.Error(err),
			)
		} else {
			result.StoresUpserted = len(dsModels)
		}
	}

	progress(80, "persisting networks")

	// ── 6. Upsert Networks ────────────────────────────────────────────────────
	if len(snap.Networks) > 0 {
		netModels := make([]model.Network, 0, len(snap.Networks))
		for _, net := range snap.Networks {
			netModels = append(netModels, model.Network{
				HypervisorID: id,
				ProviderID:   net.ProviderID,
				Name:         net.Name,
				Type:         net.Type,
				VLAN:         net.VLAN,
				Accessible:   net.Accessible,
			})
		}
		if err := s.networks.BulkUpsert(ctx, netModels); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("network upsert: %v", err))
			s.log.Error("inventory sync: network upsert failed",
				logger.String("hypervisor_id", id),
				logger.Error(err),
			)
		} else {
			result.NetworksUpserted = len(netModels)
		}
	}

	progress(85, "persisting clusters")

	// ── 7. Upsert Clusters ────────────────────────────────────────────────────
	if len(snap.Clusters) > 0 && s.clusters != nil {
		clusterModels := make([]model.Cluster, 0, len(snap.Clusters))
		for _, c := range snap.Clusters {
			clusterModels = append(clusterModels, model.Cluster{
				HypervisorID:  id,
				ProviderID:    c.ProviderID,
				Name:          c.Name,
				TotalCPU:      c.TotalCPU,
				TotalMemoryMB: c.TotalMemoryMB,
				HostCount:     c.HostCount,
			})
		}
		if err := s.clusters.BulkUpsert(ctx, clusterModels); err != nil {
			s.log.Warn("inventory sync: cluster upsert failed",
				logger.String("hypervisor_id", id),
				logger.Error(err),
			)
		}
	}

	progress(88, "persisting hosts")

	// ── 8. Upsert Hosts ───────────────────────────────────────────────────────
	if len(snap.Hosts) > 0 && s.hosts != nil {
		// Build cluster providerID → DB ID map for FK resolution
		clusterProviderToID := map[string]string{}
		if s.clusters != nil {
			existingClusters, err := s.clusters.List(ctx, id)
			if err == nil {
				for _, c := range existingClusters {
					clusterProviderToID[c.ProviderID] = c.ID.String()
				}
			}
		}

		hostModels := make([]model.Host, 0, len(snap.Hosts))
		for _, h := range snap.Hosts {
			host := model.Host{
				HypervisorID:      id,
				ProviderID:        h.ProviderID,
				Name:              h.Name,
				Status:            h.Status,
				CPUModel:          h.CPUModel,
				CPUSockets:        h.CPUSockets,
				CPUCores:          h.CPUCores,
				CPUThreads:        h.CPUThreads,
				CPUUsageMHz:       h.CPUUsageMHz,
				TotalMemoryMB:     h.TotalMemoryMB,
				UsedMemoryMB:      h.UsedMemoryMB,
				HypervisorVersion: h.HypervisorVersion,
				UptimeSeconds:     h.UptimeSeconds,
			}
			if h.ClusterProviderID != "" {
				if clusterID, ok := clusterProviderToID[h.ClusterProviderID]; ok {
					host.ClusterID = &clusterID
				}
			}
			hostModels = append(hostModels, host)
		}
		if err := s.hosts.BulkUpsert(ctx, hostModels); err != nil {
			s.log.Warn("inventory sync: host upsert failed",
				logger.String("hypervisor_id", id),
				logger.Error(err),
			)
		} else {
			// Update VM counts on hosts
			_ = s.hosts.UpdateVMCount(ctx, id)
		}
	}

	progress(90, "publishing sync event")

	// ── 7. Publish inventory.synced event ─────────────────────────────────────
	s.eventBus.Publish(ctx, messaging.Event{
		Type: messaging.EventInventorySynced,
		Payload: map[string]interface{}{
			"hypervisor_id":     id,
			"hypervisor_name":   h.Name,
			"provider":          string(h.Provider),
			"vms_updated":       result.VMsUpdated,
			"vms_removed":       result.VMsRemoved,
			"stores_upserted":   result.StoresUpserted,
			"networks_upserted": result.NetworksUpserted,
			"synced_at":         snap.SyncedAt.Format(time.RFC3339),
			"errors":            result.Errors,
		},
	})

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionExecute,
		Resource:    "hypervisor",
		ResourceID:  id,
		Description: fmt.Sprintf("inventory sync: %d VMs, %d removed", result.VMsUpdated, result.VMsRemoved),
		Success:     len(result.Errors) == 0,
	})

	// Update provider health snapshot with sync result
	syncStatus := "success"
	if len(result.Errors) > 0 {
		syncStatus = "failed"
	}
	go s.updateHealthAfterSync(context.Background(), id, syncStatus, result.VMsUpdated)

	s.log.Info("inventory sync complete",
		logger.String("hypervisor_id", id),
		logger.Int("vms_updated", result.VMsUpdated),
		logger.Int("vms_removed", result.VMsRemoved),
		logger.Int("stores", result.StoresUpserted),
		logger.Int("networks", result.NetworksUpserted),
	)

	progress(100, "sync complete")
	return result, nil
}

// ── VM mapping ────────────────────────────────────────────────────────────────

// mapVMInfoToModel converts a provider VMInfo DTO to a model.VM for persistence.
func mapVMInfoToModel(info port.VMInfo, hypervisorID uuid.UUID) model.VM {
	vm := model.VM{
		HypervisorID: hypervisorID,
		ProviderVMID: info.ProviderVMID,
		Name:         info.Name,
		Status:       info.Status,
		CPUCount:     info.CPUCount,
		MemoryMB:     info.MemoryMB,
		DiskGB:       info.DiskGB,
		MACAddress:   info.MACAddress,
		NetworkName:  info.NetworkName,
		GuestOS:      info.GuestOS,
		GuestOSType:  info.GuestOSType,
		ToolsStatus:  info.ToolsStatus,
	}

	if len(info.IPAddresses) > 0 {
		vm.IPAddresses = model.StringArray(info.IPAddresses)
	}

	// Store provider-specific extras in metadata.
	if len(info.Extra) > 0 {
		vm.Metadata = model.JSONMap{}
		for k, v := range info.Extra {
			vm.Metadata[k] = v
		}
	}

	return vm
}

// ── Credential helpers ────────────────────────────────────────────────────────

// buildCredentials decrypts the stored secret and assembles port.Credentials.
func (s *hypervisorService) buildCredentials(h *model.Hypervisor) (port.Credentials, error) {
	return buildHypervisorCredentials(h)
}

// buildHypervisorCredentials is a package-level helper so other services
// (e.g. vmService) can build credentials without depending on hypervisorService.
func buildHypervisorCredentials(h *model.Hypervisor) (port.Credentials, error) {
	secret, err := decryptSecret(h.EncryptedSecret)
	if err != nil {
		// Fall back to raw value if decryption fails (legacy placeholder)
		secret = h.EncryptedSecret
	}

	creds := port.Credentials{
		Host:      h.Host,
		Port:      h.Port,
		Username:  h.Username,
		Password:  secret,
		TLSVerify: h.TLSVerify,
	}

	// For Proxmox, pass the API token via the Token field
	if h.Provider == model.ProviderProxmox {
		if h.Metadata != nil {
			if tokenID, ok := h.Metadata["api_token_id"].(string); ok {
				creds.Token = tokenID
				creds.Password = secret
			}
		}
	}

	return creds, nil
}

// ── Encryption helpers ────────────────────────────────────────────────────────

// encryptSecret delegates to the crypto package.
func encryptSecret(plain string) (string, error) {
	return crypto.EncryptSecret(plain)
}

// decryptSecret delegates to the crypto package.
func decryptSecret(encoded string) (string, error) {
	return crypto.DecryptSecret(encoded)
}

// isTLSError returns true if the error is a TLS certificate verification failure.
func isTLSError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "certificate") ||
		strings.Contains(msg, "x509") ||
		strings.Contains(msg, "tls:") ||
		strings.Contains(msg, "TLS")
}

// ── Context helpers ───────────────────────────────────────────────────────────

// callerUUID extracts the authenticated user UUID from the context.
// Returns nil when called from the task engine (no user in context),
// which results in a NULL created_by — allowed by the DB schema.
func callerUUID(ctx context.Context) *uuid.UUID {
	id := middleware.UserIDFromContext(ctx)
	if id == "" {
		return nil
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return nil
	}
	return &u
}

// updateHealthAfterSync updates the provider health snapshot after a sync completes.
// Called as a goroutine — failures are logged but never bubble up.
func (s *hypervisorService) updateHealthAfterSync(ctx context.Context, hypervisorID, syncStatus string, vmCount int) {
	if s.healthRepo == nil {
		return
	}
	existing, err := s.healthRepo.GetByHypervisorID(ctx, hypervisorID)
	if err != nil {
		// No snapshot yet — nothing to update; the health engine will create one.
		return
	}
	now := time.Now().UTC()
	existing.LastSyncAt = &now
	existing.LastSyncStatus = syncStatus
	existing.VMCount = vmCount
	if syncStatus == "failed" {
		existing.SyncFailures24h++
	}
	if err := s.healthRepo.Upsert(ctx, existing); err != nil {
		s.log.Warn("failed to update health after sync",
			logger.String("hypervisor_id", hypervisorID),
			logger.Error(err),
		)
	}
}
