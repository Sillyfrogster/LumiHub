import type { ServerWebSocket } from "bun";
import { randomUUID } from "crypto";

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
    }

    unregister(ws: ServerWebSocket<unknown>): void {
        const instanceId = this.socketToInstance.get(ws);
        if (instanceId) {
            this.connections.delete(instanceId);
            this.instanceToUser.delete(instanceId);
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
        if (msg.type === "pong") return;

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
}

export const instanceManager = new InstanceConnectionManager();
