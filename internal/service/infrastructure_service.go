package service

import (
	"context"
	"fmt"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

type infrastructureService struct {
	clusters   port.ClusterRepository
	hosts      port.HostRepository
	vms        port.VMRepository
	datastores port.DataStoreRepository
	networks   port.NetworkRepository
	hypervisors port.HypervisorRepository
	log        logger.Logger
}

// NewInfrastructureService creates a new InfrastructureService.
func NewInfrastructureService(
	clusters port.ClusterRepository,
	hosts port.HostRepository,
	vms port.VMRepository,
	datastores port.DataStoreRepository,
	networks port.NetworkRepository,
	hypervisors port.HypervisorRepository,
	log logger.Logger,
) port.InfrastructureService {
	return &infrastructureService{
		clusters:    clusters,
		hosts:       hosts,
		vms:         vms,
		datastores:  datastores,
		networks:    networks,
		hypervisors: hypervisors,
		log:         log,
	}
}

// GetTree builds the full infrastructure hierarchy tree.
// Structure: Provider → Cluster → Host → (VM count)
func (s *infrastructureService) GetTree(ctx context.Context, hypervisorID string) ([]*port.InfrastructureTreeNode, error) {
	// Load hypervisors
	var hypervisors []model.Hypervisor
	if hypervisorID != "" {
		h, err := s.hypervisors.GetByID(ctx, hypervisorID)
		if err != nil {
			return nil, fmt.Errorf("hypervisor not found: %w", err)
		}
		hypervisors = []model.Hypervisor{*h}
	} else {
		result, err := s.hypervisors.List(ctx, port.Page{Number: 1, Size: 1000})
		if err != nil {
			return nil, fmt.Errorf("listing hypervisors: %w", err)
		}
		hypervisors = result.Items
	}

	var tree []*port.InfrastructureTreeNode

	for _, hv := range hypervisors {
		hvNode := &port.InfrastructureTreeNode{
			ID:           hv.ID.String(),
			Type:         "provider",
			Name:         hv.Name,
			Status:       string(hv.ConnectionStatus),
			ProviderType: string(hv.Provider),
			Metadata: map[string]interface{}{
				"host":     hv.Host,
				"port":     hv.Port,
				"provider": string(hv.Provider),
			},
		}

		// Load clusters for this hypervisor
		clusters, err := s.clusters.List(ctx, hv.ID.String())
		if err != nil {
			s.log.Warn("infrastructure tree: failed to load clusters",
				logger.String("hypervisor_id", hv.ID.String()),
				logger.Error(err),
			)
			clusters = nil
		}

		// Load all hosts for this hypervisor
		hosts, err := s.hosts.List(ctx, hv.ID.String())
		if err != nil {
			s.log.Warn("infrastructure tree: failed to load hosts",
				logger.String("hypervisor_id", hv.ID.String()),
				logger.Error(err),
			)
			hosts = nil
		}

		// Build cluster ID → cluster node map
		clusterNodeMap := make(map[string]*port.InfrastructureTreeNode)
		for i := range clusters {
			c := &clusters[i]
			clusterNode := &port.InfrastructureTreeNode{
				ID:      c.ID.String(),
				Type:    "cluster",
				Name:    c.Name,
				Status:  "active",
				VMCount: c.VMCount,
				Metadata: map[string]interface{}{
					"provider_id":     c.ProviderID,
					"total_cpu":       c.TotalCPU,
					"total_memory_mb": c.TotalMemoryMB,
					"host_count":      c.HostCount,
				},
			}
			clusterNodeMap[c.ID.String()] = clusterNode
			hvNode.Children = append(hvNode.Children, clusterNode)
		}

		// Attach hosts to clusters (or directly to hypervisor if no cluster)
		for i := range hosts {
			h := &hosts[i]
			hostNode := &port.InfrastructureTreeNode{
				ID:      h.ID.String(),
				Type:    "host",
				Name:    h.Name,
				Status:  h.Status,
				VMCount: h.VMCount,
				Metadata: map[string]interface{}{
					"provider_id":       h.ProviderID,
					"cpu_model":         h.CPUModel,
					"cpu_cores":         h.CPUCores,
					"cpu_sockets":       h.CPUSockets,
					"total_memory_mb":   h.TotalMemoryMB,
					"used_memory_mb":    h.UsedMemoryMB,
					"hypervisor_version": h.HypervisorVersion,
					"uptime_seconds":    h.UptimeSeconds,
				},
			}
			hvNode.VMCount += h.VMCount

			if h.ClusterID != nil {
				if clusterNode, ok := clusterNodeMap[*h.ClusterID]; ok {
					clusterNode.Children = append(clusterNode.Children, hostNode)
					continue
				}
			}
			// No cluster — attach directly to hypervisor
			hvNode.Children = append(hvNode.Children, hostNode)
		}

		// If no clusters and no hosts, still show the hypervisor node
		tree = append(tree, hvNode)
	}

	return tree, nil
}

// ListHosts returns all hosts, optionally filtered by hypervisor.
func (s *infrastructureService) ListHosts(ctx context.Context, hypervisorID string) ([]model.Host, error) {
	return s.hosts.List(ctx, hypervisorID)
}

// GetHost returns a single host with its VMs, datastores, and networks.
func (s *infrastructureService) GetHost(ctx context.Context, id string) (*port.HostDetail, error) {
	host, err := s.hosts.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("host not found: %w", err)
	}

	// Load VMs for this host — match by esxi_host or node metadata
	vmResult, err := s.vms.List(ctx, port.VMFilter{HypervisorID: host.HypervisorID}, port.Page{Number: 1, Size: 1000})
	var hostedVMs []model.VM
	if err == nil {
		for _, vm := range vmResult.Items {
			esxiHost, _ := vm.Metadata["esxi_host"].(string)
			node, _ := vm.Metadata["node"].(string)
			if esxiHost == host.Name || node == host.Name {
				hostedVMs = append(hostedVMs, vm)
			}
		}
	}

	// Load datastores for this hypervisor
	datastores, err := s.datastores.List(ctx, host.HypervisorID)
	if err != nil {
		datastores = nil
	}

	// Load networks for this hypervisor
	networks, err := s.networks.List(ctx, host.HypervisorID)
	if err != nil {
		networks = nil
	}

	return &port.HostDetail{
		Host:       *host,
		VMs:        hostedVMs,
		DataStores: datastores,
		Networks:   networks,
	}, nil
}

// ListClusters returns all clusters, optionally filtered by hypervisor.
func (s *infrastructureService) ListClusters(ctx context.Context, hypervisorID string) ([]model.Cluster, error) {
	return s.clusters.List(ctx, hypervisorID)
}

// GetCluster returns a single cluster with its hosts.
func (s *infrastructureService) GetCluster(ctx context.Context, id string) (*model.Cluster, error) {
	return s.clusters.GetByID(ctx, id)
}

// ListDataStores returns all datastores, optionally filtered by hypervisor.
func (s *infrastructureService) ListDataStores(ctx context.Context, hypervisorID string) ([]model.DataStore, error) {
	return s.datastores.List(ctx, hypervisorID)
}

// ListNetworks returns all virtual networks, optionally filtered by hypervisor.
func (s *infrastructureService) ListNetworks(ctx context.Context, hypervisorID string) ([]model.Network, error) {
	return s.networks.List(ctx, hypervisorID)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetTopology — builds a graph of nodes and edges for topology visualization
// ─────────────────────────────────────────────────────────────────────────────

func (s *infrastructureService) GetTopology(ctx context.Context, hypervisorID string) (*port.TopologyGraph, error) {
	graph := &port.TopologyGraph{}

	// Load hypervisors
	var hypervisors []model.Hypervisor
	if hypervisorID != "" {
		h, err := s.hypervisors.GetByID(ctx, hypervisorID)
		if err != nil {
			return nil, fmt.Errorf("hypervisor not found: %w", err)
		}
		hypervisors = []model.Hypervisor{*h}
	} else {
		result, err := s.hypervisors.List(ctx, port.Page{Number: 1, Size: 1000})
		if err != nil {
			return nil, fmt.Errorf("listing hypervisors: %w", err)
		}
		hypervisors = result.Items
	}

	for _, hv := range hypervisors {
		hvID := hv.ID.String()
		hvStatus := string(hv.ConnectionStatus)
		if hvStatus == "" {
			hvStatus = "unknown"
		}

		// Provider node
		graph.Nodes = append(graph.Nodes, port.TopologyNode{
			ID:           hvID,
			Type:         "provider",
			Label:        hv.Name,
			Status:       hvStatus,
			ProviderType: string(hv.Provider),
			Metadata: map[string]interface{}{
				"host":     hv.Host,
				"port":     hv.Port,
				"provider": string(hv.Provider),
			},
		})

		// Clusters
		clusters, _ := s.clusters.List(ctx, hvID)
		for i := range clusters {
			c := &clusters[i]
			cID := c.ID.String()
			graph.Nodes = append(graph.Nodes, port.TopologyNode{
				ID:     cID,
				Type:   "cluster",
				Label:  c.Name,
				Status: "active",
				Metadata: map[string]interface{}{
					"total_cpu":       c.TotalCPU,
					"total_memory_mb": c.TotalMemoryMB,
					"host_count":      c.HostCount,
					"vm_count":        c.VMCount,
				},
			})
			graph.Edges = append(graph.Edges, port.TopologyEdge{
				Source: hvID,
				Target: cID,
				Type:   "contains",
			})
		}

		// Hosts
		hosts, _ := s.hosts.List(ctx, hvID)
		for i := range hosts {
			h := &hosts[i]
			hID := h.ID.String()

			// Compute memory usage pct
			var memPct float64
			if h.TotalMemoryMB > 0 {
				memPct = float64(h.UsedMemoryMB) / float64(h.TotalMemoryMB) * 100
			}

			hostStatus := "healthy"
			if h.Status == "disconnected" || h.Status == "error" {
				hostStatus = "critical"
			} else if h.Status == "maintenance" {
				hostStatus = "warning"
			} else if memPct > 85 {
				hostStatus = "warning"
			}

			graph.Nodes = append(graph.Nodes, port.TopologyNode{
				ID:          hID,
				Type:        "host",
				Label:       h.Name,
				Status:      hostStatus,
				VMCount:     h.VMCount,
				MemUsagePct: memPct,
				Metadata: map[string]interface{}{
					"cpu_cores":         h.CPUCores,
					"total_memory_mb":   h.TotalMemoryMB,
					"used_memory_mb":    h.UsedMemoryMB,
					"hypervisor_version": h.HypervisorVersion,
					"status":            h.Status,
				},
			})

			// Edge: cluster → host or provider → host
			if h.ClusterID != nil {
				graph.Edges = append(graph.Edges, port.TopologyEdge{
					Source: *h.ClusterID,
					Target: hID,
					Type:   "contains",
				})
			} else {
				graph.Edges = append(graph.Edges, port.TopologyEdge{
					Source: hvID,
					Target: hID,
					Type:   "contains",
				})
			}
		}

		// Datastores
		datastores, _ := s.datastores.List(ctx, hvID)
		for i := range datastores {
			ds := &datastores[i]
			dsID := ds.ID.String()
			var diskPct float64
			if ds.CapacityGB > 0 {
				diskPct = ds.UsedGB / ds.CapacityGB * 100
			}
			dsStatus := "healthy"
			if diskPct > 90 {
				dsStatus = "critical"
			} else if diskPct > 75 {
				dsStatus = "warning"
			}
			graph.Nodes = append(graph.Nodes, port.TopologyNode{
				ID:           dsID,
				Type:         "datastore",
				Label:        ds.Name,
				Status:       dsStatus,
				DiskUsagePct: diskPct,
				Metadata: map[string]interface{}{
					"type":        ds.Type,
					"capacity_gb": ds.CapacityGB,
					"used_gb":     ds.UsedGB,
					"free_gb":     ds.FreeGB,
					"accessible":  ds.Accessible,
				},
			})
			graph.Edges = append(graph.Edges, port.TopologyEdge{
				Source: hvID,
				Target: dsID,
				Type:   "contains",
			})
		}

		// Networks
		networks, _ := s.networks.List(ctx, hvID)
		for i := range networks {
			net := &networks[i]
			netID := net.ID.String()
			netStatus := "healthy"
			if !net.Accessible {
				netStatus = "critical"
			}
			graph.Nodes = append(graph.Nodes, port.TopologyNode{
				ID:     netID,
				Type:   "network",
				Label:  net.Name,
				Status: netStatus,
				Metadata: map[string]interface{}{
					"type":       net.Type,
					"vlan":       net.VLAN,
					"accessible": net.Accessible,
				},
			})
			graph.Edges = append(graph.Edges, port.TopologyEdge{
				Source: hvID,
				Target: netID,
				Type:   "contains",
			})
		}

		// VM → host edges (relationship mapping)
		vmResult, err := s.vms.List(ctx, port.VMFilter{HypervisorID: hvID}, port.Page{Number: 1, Size: 5000})
		if err == nil {
			// Build host name → ID map
			hostNameToID := make(map[string]string, len(hosts))
			for i := range hosts {
				hostNameToID[hosts[i].Name] = hosts[i].ID.String()
			}
			for _, vm := range vmResult.Items {
				esxiHost, _ := vm.Metadata["esxi_host"].(string)
				node, _ := vm.Metadata["node"].(string)
				hostName := esxiHost
				if hostName == "" {
					hostName = node
				}
				if hostID, ok := hostNameToID[hostName]; ok {
					graph.Edges = append(graph.Edges, port.TopologyEdge{
						Source: hostID,
						Target: vm.ID.String(),
						Type:   "runs_on",
					})
				}
			}
		}
	}

	return graph, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetHeatmap — resource utilisation heatmaps per host and datastore
// ─────────────────────────────────────────────────────────────────────────────

func (s *infrastructureService) GetHeatmap(ctx context.Context, hypervisorID string) (*port.InfraHeatmap, error) {
	heatmap := &port.InfraHeatmap{}

	var hypervisors []model.Hypervisor
	if hypervisorID != "" {
		h, err := s.hypervisors.GetByID(ctx, hypervisorID)
		if err != nil {
			return nil, fmt.Errorf("hypervisor not found: %w", err)
		}
		hypervisors = []model.Hypervisor{*h}
	} else {
		result, err := s.hypervisors.List(ctx, port.Page{Number: 1, Size: 1000})
		if err != nil {
			return nil, fmt.Errorf("listing hypervisors: %w", err)
		}
		hypervisors = result.Items
	}

	for _, hv := range hypervisors {
		hvID := hv.ID.String()

		hosts, _ := s.hosts.List(ctx, hvID)
		for i := range hosts {
			h := &hosts[i]

			// Memory heatmap
			var memPct float64
			if h.TotalMemoryMB > 0 {
				memPct = float64(h.UsedMemoryMB) / float64(h.TotalMemoryMB) * 100
			}
			memStatus := heatmapStatus(memPct)

			// CPU heatmap — use CPUUsageMHz as proxy (no total MHz stored, use cores * 2000 as estimate)
			totalMHz := h.CPUCores * h.CPUSockets * 2000
			var cpuPct float64
			if totalMHz > 0 {
				cpuPct = float64(h.CPUUsageMHz) / float64(totalMHz) * 100
			}
			cpuStatus := heatmapStatus(cpuPct)

			meta := map[string]interface{}{
				"hypervisor_name": hv.Name,
				"provider":        string(hv.Provider),
				"cpu_cores":       h.CPUCores,
				"total_memory_mb": h.TotalMemoryMB,
				"used_memory_mb":  h.UsedMemoryMB,
				"status":          h.Status,
			}

			heatmap.CPU = append(heatmap.CPU, port.HeatmapCell{
				ID:           h.ID.String(),
				Label:        h.Name,
				HypervisorID: hvID,
				Value:        cpuPct,
				Status:       cpuStatus,
				VMCount:      h.VMCount,
				Metadata:     meta,
			})
			heatmap.Memory = append(heatmap.Memory, port.HeatmapCell{
				ID:           h.ID.String(),
				Label:        h.Name,
				HypervisorID: hvID,
				Value:        memPct,
				Status:       memStatus,
				VMCount:      h.VMCount,
				Metadata:     meta,
			})

			// VM density heatmap — VMs per host relative to max across all hosts
			heatmap.VMDensity = append(heatmap.VMDensity, port.HeatmapCell{
				ID:           h.ID.String(),
				Label:        h.Name,
				HypervisorID: hvID,
				Value:        float64(h.VMCount), // raw count; frontend normalises
				Status:       "healthy",
				VMCount:      h.VMCount,
				Metadata:     meta,
			})
		}

		// Datastore heatmap
		datastores, _ := s.datastores.List(ctx, hvID)
		for i := range datastores {
			ds := &datastores[i]
			var diskPct float64
			if ds.CapacityGB > 0 {
				diskPct = ds.UsedGB / ds.CapacityGB * 100
			}
			heatmap.Datastore = append(heatmap.Datastore, port.HeatmapCell{
				ID:           ds.ID.String(),
				Label:        ds.Name,
				HypervisorID: hvID,
				Value:        diskPct,
				Status:       heatmapStatus(diskPct),
				Metadata: map[string]interface{}{
					"hypervisor_name": hv.Name,
					"provider":        string(hv.Provider),
					"type":            ds.Type,
					"capacity_gb":     ds.CapacityGB,
					"used_gb":         ds.UsedGB,
					"free_gb":         ds.FreeGB,
				},
			})
		}
	}

	// Normalise VM density to 0–100
	if len(heatmap.VMDensity) > 0 {
		maxVMs := 0
		for _, c := range heatmap.VMDensity {
			if c.VMCount > maxVMs {
				maxVMs = c.VMCount
			}
		}
		if maxVMs > 0 {
			for i := range heatmap.VMDensity {
				heatmap.VMDensity[i].Value = float64(heatmap.VMDensity[i].VMCount) / float64(maxVMs) * 100
				heatmap.VMDensity[i].Status = heatmapStatus(heatmap.VMDensity[i].Value)
			}
		}
	}

	return heatmap, nil
}

func heatmapStatus(pct float64) string {
	if pct >= 85 {
		return "critical"
	}
	if pct >= 65 {
		return "warning"
	}
	return "healthy"
}

// ─────────────────────────────────────────────────────────────────────────────
// GetSummary — live operations dashboard summary
// ─────────────────────────────────────────────────────────────────────────────

func (s *infrastructureService) GetSummary(ctx context.Context) (*port.InfraSummary, error) {
	summary := &port.InfraSummary{}

	// Load all hypervisors
	hvResult, err := s.hypervisors.List(ctx, port.Page{Number: 1, Size: 1000})
	if err != nil {
		return nil, fmt.Errorf("listing hypervisors: %w", err)
	}
	hypervisors := hvResult.Items
	summary.TotalProviders = len(hypervisors)

	// Load all VMs
	vmResult, err := s.vms.List(ctx, port.VMFilter{}, port.Page{Number: 1, Size: 10000})
	if err != nil {
		return nil, fmt.Errorf("listing vms: %w", err)
	}
	vms := vmResult.Items
	summary.TotalVMs = len(vms)

	// Count stale snapshots (VMs with > 5 snapshots)
	staleSnaps := 0
	for _, vm := range vms {
		if vm.Status == model.VMStatusRunning {
			summary.RunningVMs++
		}
		// Count snapshots via metadata proxy (actual count requires snapshot repo)
	}
	_ = staleSnaps

	now := time.Now().UTC()

	for _, hv := range hypervisors {
		hvID := hv.ID.String()

		// Provider health
		if hv.ConnectionStatus == model.ConnectionStatusConnected {
			summary.HealthyProviders++
		} else if hv.ConnectionStatus == model.ConnectionStatusError ||
			hv.ConnectionStatus == model.ConnectionStatusDisconnected {
			summary.UnhealthyProviders++
			summary.Alerts = append(summary.Alerts, port.OperationsAlert{
				ID:           "provider-" + hvID,
				Severity:     "critical",
				Category:     "provider",
				Title:        "Provider Disconnected: " + hv.Name,
				Description:  fmt.Sprintf("Hypervisor '%s' (%s) is %s", hv.Name, string(hv.Provider), string(hv.ConnectionStatus)),
				ResourceID:   hvID,
				ResourceType: "hypervisor",
				HypervisorID: hvID,
			})
		}

		// Hosts
		hosts, _ := s.hosts.List(ctx, hvID)
		summary.TotalHosts += len(hosts)

		hostCount := 0
		clusterCount := 0
		runningVMs := 0
		overloadedHosts := 0

		for _, h := range hosts {
			if h.Status == "connected" {
				summary.ConnectedHosts++
			} else if h.Status == "disconnected" || h.Status == "error" {
				summary.DisconnectedHosts++
				summary.Alerts = append(summary.Alerts, port.OperationsAlert{
					ID:           "host-" + h.ID.String(),
					Severity:     "warning",
					Category:     "host",
					Title:        "Host Disconnected: " + h.Name,
					Description:  fmt.Sprintf("Host '%s' on %s is %s", h.Name, hv.Name, h.Status),
					ResourceID:   h.ID.String(),
					ResourceType: "host",
					HypervisorID: hvID,
				})
			}

			// Check memory overload
			if h.TotalMemoryMB > 0 {
				memPct := float64(h.UsedMemoryMB) / float64(h.TotalMemoryMB) * 100
				if memPct > 85 {
					overloadedHosts++
					summary.OverloadedHosts++
					summary.Alerts = append(summary.Alerts, port.OperationsAlert{
						ID:           "host-mem-" + h.ID.String(),
						Severity:     "warning",
						Category:     "host",
						Title:        fmt.Sprintf("Host Overloaded: %s (%.0f%% memory)", h.Name, memPct),
						Description:  fmt.Sprintf("Host '%s' memory usage is %.1f%% (%d/%d MB)", h.Name, memPct, h.UsedMemoryMB, h.TotalMemoryMB),
						ResourceID:   h.ID.String(),
						ResourceType: "host",
						HypervisorID: hvID,
					})
				}
			}
			hostCount++
		}

		// Clusters
		clusters, _ := s.clusters.List(ctx, hvID)
		summary.TotalClusters += len(clusters)
		clusterCount = len(clusters)

		// Datastores
		datastores, _ := s.datastores.List(ctx, hvID)
		summary.TotalDatastores += len(datastores)
		for _, ds := range datastores {
			if ds.CapacityGB > 0 {
				pct := ds.UsedGB / ds.CapacityGB * 100
				if pct > 90 {
					summary.Alerts = append(summary.Alerts, port.OperationsAlert{
						ID:           "ds-" + ds.ID.String(),
						Severity:     "critical",
						Category:     "capacity",
						Title:        fmt.Sprintf("Storage Critical: %s (%.0f%% used)", ds.Name, pct),
						Description:  fmt.Sprintf("Datastore '%s' on %s is %.1f%% full (%.1f GB free)", ds.Name, hv.Name, pct, ds.FreeGB),
						ResourceID:   ds.ID.String(),
						ResourceType: "datastore",
						HypervisorID: hvID,
					})
				} else if pct > 75 {
					summary.Alerts = append(summary.Alerts, port.OperationsAlert{
						ID:           "ds-warn-" + ds.ID.String(),
						Severity:     "warning",
						Category:     "capacity",
						Title:        fmt.Sprintf("Storage Warning: %s (%.0f%% used)", ds.Name, pct),
						Description:  fmt.Sprintf("Datastore '%s' on %s is %.1f%% full (%.1f GB free)", ds.Name, hv.Name, pct, ds.FreeGB),
						ResourceID:   ds.ID.String(),
						ResourceType: "datastore",
						HypervisorID: hvID,
					})
				}
			}
		}

		// Networks
		networks, _ := s.networks.List(ctx, hvID)
		summary.TotalNetworks += len(networks)

		// Count running VMs for this hypervisor
		for _, vm := range vms {
			if vm.HypervisorID.String() == hvID && vm.Status == model.VMStatusRunning {
				runningVMs++
			}
		}

		// Check for stale sync (last_checked_at > 24h ago)
		var lastSyncAt *time.Time
		if hv.LastCheckedAt != nil {
			lastSyncAt = hv.LastCheckedAt
			if now.Sub(*hv.LastCheckedAt) > 24*time.Hour {
				summary.Alerts = append(summary.Alerts, port.OperationsAlert{
					ID:           "sync-" + hvID,
					Severity:     "warning",
					Category:     "sync",
					Title:        "Stale Sync: " + hv.Name,
					Description:  fmt.Sprintf("Hypervisor '%s' has not been synced in over 24 hours", hv.Name),
					ResourceID:   hvID,
					ResourceType: "hypervisor",
					HypervisorID: hvID,
				})
			}
		}

		// Per-provider summary
		var memPct, cpuPct, storagePct float64
		for _, h := range hosts {
			if h.TotalMemoryMB > 0 {
				memPct += float64(h.UsedMemoryMB) / float64(h.TotalMemoryMB) * 100
			}
		}
		if len(hosts) > 0 {
			memPct /= float64(len(hosts))
		}
		for _, ds := range datastores {
			if ds.CapacityGB > 0 {
				storagePct += ds.UsedGB / ds.CapacityGB * 100
			}
		}
		if len(datastores) > 0 {
			storagePct /= float64(len(datastores))
		}

		provStatus := string(hv.ConnectionStatus)
		if provStatus == "" {
			provStatus = "unknown"
		}

		summary.Providers = append(summary.Providers, port.ProviderSummary{
			HypervisorID:    hvID,
			HypervisorName:  hv.Name,
			Provider:        string(hv.Provider),
			Status:          provStatus,
			VMCount:         len(vms),
			RunningVMs:      runningVMs,
			HostCount:       hostCount,
			ClusterCount:    clusterCount,
			CPUUsagePct:     cpuPct,
			MemUsagePct:     memPct,
			StorageUsagePct: storagePct,
			OverloadedHosts: overloadedHosts,
			LastSyncAt:      lastSyncAt,
		})
	}

	// Count alerts by severity
	for _, a := range summary.Alerts {
		if a.Severity == "critical" {
			summary.CriticalAlerts++
		} else if a.Severity == "warning" {
			summary.WarningAlerts++
		}
	}

	// VM-host relationship count
	summary.VMHostRelationships = summary.TotalVMs

	return summary, nil
}
