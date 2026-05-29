import api from "./client";
import type { ApiResponse } from "@/types";

export interface SystemHealthData {
  timestamp: string;
  uptime_secs: number;
  api: { status: string };
  database: {
    status: string;
    open_connections: number;
    idle_connections: number;
    in_use_connections: number;
    wait_count: number;
    latency_ms: number;
  };
  cache: {
    status: string;
    used_memory_mb: number;
    hit_rate_percent: number;
    latency_ms: number;
  };
  tasks: {
    pending_tasks: number;
    running_tasks: number;
    queue_depths: Record<string, number>;
    total_queued: number;
  };
  runtime: {
    goroutines: number;
    heap_alloc_mb: number;
    heap_sys_mb: number;
    gc_pause_ms: number;
    num_gc: number;
  };
}

export const systemApi = {
  getHealth: () =>
    api
      .get<ApiResponse<SystemHealthData>>("/v1/system/health")
      .then((r) => r.data.data),
};
