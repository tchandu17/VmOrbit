type WSMessage = { type: string; payload: unknown };
type Callback = (msg: WSMessage) => void;

// ─────────────────────────────────────────────────────────────────────────────
// Message deduplication — drops identical messages within a short window
// to prevent WS flooding from rapid backend events.
// ─────────────────────────────────────────────────────────────────────────────
const DEDUP_WINDOW_MS = 200;

class MessageDeduplicator {
  private seen = new Map<string, number>();

  isDuplicate(msg: WSMessage): boolean {
    const key = msg.type + ":" + JSON.stringify(msg.payload);
    const now = Date.now();
    const last = this.seen.get(key);
    if (last !== undefined && now - last < DEDUP_WINDOW_MS) return true;
    this.seen.set(key, now);
    // Prune old entries every 100 inserts to prevent unbounded growth
    if (this.seen.size > 500) {
      for (const [k, t] of this.seen) {
        if (now - t > DEDUP_WINDOW_MS * 10) this.seen.delete(k);
      }
    }
    return false;
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Per-room throttle — limits how often a room's callbacks are invoked.
// Useful for high-frequency rooms like "tasks" during bulk operations.
// ─────────────────────────────────────────────────────────────────────────────
const ROOM_THROTTLE_MS: Record<string, number> = {
  tasks: 100,    // max 10 task updates/sec
  vms: 250,      // max 4 VM updates/sec
  inventory: 500, // max 2 inventory events/sec
};

class RoomThrottle {
  private lastFired = new Map<string, number>();
  private pending = new Map<string, { msg: WSMessage; timer: ReturnType<typeof setTimeout> }>();

  shouldFire(room: string, msg: WSMessage, fire: () => void): void {
    const throttleMs = ROOM_THROTTLE_MS[room];
    if (!throttleMs) {
      fire();
      return;
    }

    const now = Date.now();
    const last = this.lastFired.get(room) ?? 0;
    const elapsed = now - last;

    // Cancel any pending deferred fire for this room
    const existing = this.pending.get(room);
    if (existing) {
      clearTimeout(existing.timer);
      this.pending.delete(room);
    }

    if (elapsed >= throttleMs) {
      // Fire immediately
      this.lastFired.set(room, now);
      fire();
    } else {
      // Defer to end of throttle window, keeping only the latest message
      const delay = throttleMs - elapsed;
      const timer = setTimeout(() => {
        this.lastFired.set(room, Date.now());
        this.pending.delete(room);
        fire();
      }, delay);
      this.pending.set(room, { msg, timer });
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// WSClient
// ─────────────────────────────────────────────────────────────────────────────

class WSClient {
  private ws: WebSocket | null = null;
  private rooms = new Map<string, Set<Callback>>();
  private reconnectDelay = 1000;
  private token: string | null = null;
  private connected = false;
  private onConnectCallbacks: Array<() => void> = [];
  private onDisconnectCallbacks: Array<() => void> = [];
  private dedup = new MessageDeduplicator();
  private throttle = new RoomThrottle();

  connect(token: string) {
    this.token = token;
    const backendHost =
      typeof window !== "undefined" ? window.location.hostname : "localhost";
    const wsUrl = `ws://${backendHost}:8080/ws?token=${token}`;
    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      this.connected = true;
      this.reconnectDelay = 1000;
      this.onConnectCallbacks.forEach((cb) => cb());
      // Re-subscribe to all rooms after reconnect
      this.rooms.forEach((_, room) => this.sendSubscribe(room));
    };

    this.ws.onmessage = (e) => {
      try {
        const msg: WSMessage = JSON.parse(e.data as string);
        // Drop duplicate messages within the dedup window
        if (this.dedup.isDuplicate(msg)) return;
        this.dispatch(msg);
      } catch {
        // ignore malformed frames
      }
    };

    this.ws.onclose = () => {
      this.connected = false;
      this.onDisconnectCallbacks.forEach((cb) => cb());
      setTimeout(() => {
        if (this.token) this.connect(this.token);
      }, this.reconnectDelay);
      // Exponential backoff: 1s → 2s → 4s → … → 30s
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30_000);
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  disconnect() {
    this.token = null;
    this.ws?.close();
    this.ws = null;
  }

  subscribe(room: string, cb: Callback): () => void {
    if (!this.rooms.has(room)) this.rooms.set(room, new Set());
    this.rooms.get(room)!.add(cb);
    if (this.connected) this.sendSubscribe(room);
    return () => {
      this.rooms.get(room)?.delete(cb);
    };
  }

  onConnect(cb: () => void) {
    this.onConnectCallbacks.push(cb);
  }

  onDisconnect(cb: () => void) {
    this.onDisconnectCallbacks.push(cb);
  }

  isConnected() {
    return this.connected;
  }

  private sendSubscribe(room: string) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: "subscribe", room }));
    }
  }

  private dispatch(msg: WSMessage) {
    const type = msg.type as string;

    // Route to the correct rooms based on event type prefix
    const roomsToNotify: string[] = [];

    if (type.startsWith("task.") || type.startsWith("inventory.")) {
      roomsToNotify.push("tasks");
      const payload = msg.payload as Record<string, unknown>;
      if (payload?.task_id) {
        roomsToNotify.push(`task:${payload.task_id}`);
      }
    }
    if (type.startsWith("vm.")) {
      roomsToNotify.push("vms");
    }
    if (type.startsWith("hypervisor.") || type === "inventory.synced") {
      roomsToNotify.push("inventory");
    }
    if (type === "platform.event") {
      roomsToNotify.push("events");
    }

    for (const room of roomsToNotify) {
      const callbacks = this.rooms.get(room);
      if (!callbacks || callbacks.size === 0) continue;

      this.throttle.shouldFire(room, msg, () => {
        callbacks.forEach((cb) => cb(msg));
      });
    }
  }
}

export const wsClient = new WSClient();
