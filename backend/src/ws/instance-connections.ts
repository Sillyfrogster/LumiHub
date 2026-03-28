import type { ServerWebSocket } from "bun";
import { randomUUID } from "crypto";
import { logger } from "../utils/logger.ts";

const HEARTBEAT_INTERVAL_MS = 30_000;
const HEARTBEAT_TIMEOUT_MS = 10_000;

export interface WSMessage {
    type: string;
    id: string;
    payload?: unknown;
    timestamp: number;
    replyTo?: string;
}

interface PendingRequest {
    resolve: (msg: WSMessage) => void;
    reject: (err: Error) => void;
    timeout: ReturnType<typeof setTimeout>;
}

class InstanceConnectionManager {
    /** instanceId -> WebSocket */
    private connections = new Map<string, ServerWebSocket<unknown>>();
    /** WebSocket -> instanceId */
    private socketToInstance = new Map<ServerWebSocket<unknown>, string>();
    /** instanceId -> userId (for lookup) */
    private instanceToUser = new Map<string, string>();
    /** requestId -> pending request callback */
    private pendingRequests = new Map<string, PendingRequest>();
    /** instanceId -> timestamp of last pong received */
    private lastPong = new Map<string, number>();
    /** Heartbeat interval handle */
    private heartbeatInterval: ReturnType<typeof setInterval> | null = null;

    register(instanceId: string, userId: string, ws: ServerWebSocket<unknown>): void {
        // Close existing connection for this instance if any
        const existing = this.connections.get(instanceId);
        if (existing && existing !== ws) {
            try { existing.close(1000, "Replaced by new connection"); } catch {}
            this.socketToInstance.delete(existing);
        }

        this.connections.set(instanceId, ws);
        this.socketToInstance.set(ws, instanceId);
        this.instanceToUser.set(instanceId, userId);
        this.lastPong.set(instanceId, Date.now());
    }

    unregister(ws: ServerWebSocket<unknown>): void {
        const instanceId = this.socketToInstance.get(ws);
        if (instanceId) {
            this.connections.delete(instanceId);
            this.instanceToUser.delete(instanceId);
            this.lastPong.delete(instanceId);
        }
        this.socketToInstance.delete(ws);
    }

    isOnline(instanceId: string): boolean {
        return this.connections.has(instanceId);
    }

    getInstanceIdForSocket(ws: ServerWebSocket<unknown>): string | undefined {
        return this.socketToInstance.get(ws);
    }

    /** Get all online instance IDs for a given user. */
    getOnlineInstancesForUser(userId: string): string[] {
        const result: string[] = [];
        for (const [instanceId, uid] of this.instanceToUser) {
            if (uid === userId && this.connections.has(instanceId)) {
                result.push(instanceId);
            }
        }
        return result;
    }

    /** Send a message to an instance (fire-and-forget). */
    sendMessage(instanceId: string, message: WSMessage): boolean {
        const ws = this.connections.get(instanceId);
        if (!ws) return false;
        try {
            ws.send(JSON.stringify(message));
            return true;
        } catch {
            return false;
        }
    }

    /** Send a request and await a response (with timeout). */
    sendRequest(instanceId: string, type: string, payload: unknown, timeoutMs = 30000): Promise<WSMessage> {
        return new Promise((resolve, reject) => {
            const id = randomUUID();
            const message: WSMessage = { type, id, payload, timestamp: Date.now() };

            if (!this.sendMessage(instanceId, message)) {
                reject(new Error("Instance is not connected"));
                return;
            }

            const timeout = setTimeout(() => {
                this.pendingRequests.delete(id);
                reject(new Error("Request timed out"));
            }, timeoutMs);

            this.pendingRequests.set(id, { resolve, reject, timeout });
        });
    }

    /** Handle an incoming message from an instance WebSocket. */
    handleMessage(ws: ServerWebSocket<unknown>, data: string): void {
        let msg: WSMessage;
        try {
            msg = JSON.parse(data);
        } catch {
            return; // Ignore malformed messages
        }

        // Handle pong (heartbeat response)
        if (msg.type === "pong") {
            const instanceId = this.socketToInstance.get(ws);
            if (instanceId) {
                this.lastPong.set(instanceId, Date.now());
            }
            return;
        }

        // Handle responses to pending requests
        if (msg.replyTo && this.pendingRequests.has(msg.replyTo)) {
            const pending = this.pendingRequests.get(msg.replyTo)!;
            clearTimeout(pending.timeout);
            this.pendingRequests.delete(msg.replyTo);
            pending.resolve(msg);
            return;
        }

        // Handle instance_info messages
        if (msg.type === "instance_info") {
            // Could store capabilities in the future
            return;
        }

        // Handle manifest sync from Lumiverse instances
        if (msg.type === "manifest_sync") {
            const instanceId = this.socketToInstance.get(ws);
            if (instanceId && msg.payload) {
                const { entries } = msg.payload as { entries: Array<{ slug: string; type: 'character' | 'worldbook'; name: string; creator: string; source: string; installed_at: number | null }> };
                if (Array.isArray(entries)) {
                    import('../services/manifest.service.ts').then((svc) => {
                        svc.syncManifest(instanceId, entries).catch((err) => {
                            logger.warn(`[WS] Failed to sync manifest for ${instanceId}:`, err);
                        });
                    });
                }
            }
            return;
        }
    }

    /** Force-disconnect an instance. */
    disconnectInstance(instanceId: string): void {
        const ws = this.connections.get(instanceId);
        if (ws) {
            try { ws.close(1000, "Instance unlinked"); } catch {}
            this.unregister(ws);
        }
    }

    get connectionCount(): number {
        return this.connections.size;
    }

    startHeartbeat(): void {
        if (this.heartbeatInterval) return;

        this.heartbeatInterval = setInterval(() => {
            const now = Date.now();
            const staleThreshold = now - HEARTBEAT_INTERVAL_MS - HEARTBEAT_TIMEOUT_MS;

            for (const [instanceId, ws] of this.connections) {
                const last = this.lastPong.get(instanceId) ?? 0;
                if (last < staleThreshold) {
                    logger.info(`[WS] Instance ${instanceId} missed heartbeat, disconnecting`);
                    try { ws.close(1000, "Heartbeat timeout"); } catch {}
                    this.unregister(ws);
                    continue;
                }

                try {
                    ws.send(JSON.stringify({
                        type: "ping",
                        id: randomUUID(),
                        timestamp: now,
                    }));
                } catch {
                    logger.info(`[WS] Failed to send ping to ${instanceId}, disconnecting`);
                    try { ws.close(); } catch {}
                    this.unregister(ws);
                }
            }
        }, HEARTBEAT_INTERVAL_MS);
    }

    stopHeartbeat(): void {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
        }
    }
}

export const instanceManager = new InstanceConnectionManager();
