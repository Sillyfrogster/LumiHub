"use client";

import {
  Check,
  CircleAlert,
  Clock,
  RefreshCw,
  Send,
  Upload,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { browserFetch } from "@/lib/api/browser-mutation";
import type { components } from "@/lib/api/schema";
import { useAuth } from "@/lib/auth";
import styles from "./SendToInstance.module.css";

type Instance = components["schemas"]["AssetInstance"];
type Delivery = components["schemas"]["QueuedDelivery"];

// A delivery is collected within one long-poll, so the page checks back a few times and stops.
const WATCH_INTERVAL_MS = 8000;
const WATCH_LIMIT = 20;

const FAILURE_LINES: Record<string, string> = {
  withdrawn: "This asset was withdrawn before it could be collected.",
  unsupported:
    "This application accepts no format this asset can be written in.",
  abandoned: "The application kept taking this delivery without installing it.",
};

function isPending(delivery: Delivery | null): boolean {
  return delivery !== null && delivery.state !== "failed";
}

export function SendToInstance({ assetId }: { assetId: string }) {
  const { account } = useAuth();
  const [instances, setInstances] = useState<Instance[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState("");
  const watched = useRef(0);

  const read = useCallback(async () => {
    const response = await fetch(`/api/v1/assets/${assetId}/instances`, {
      cache: "no-store",
      credentials: "same-origin",
    });
    if (!response.ok) {
      setInstances([]);
      return;
    }
    const answer =
      (await response.json()) as components["schemas"]["AssetInstanceList"];
    setInstances(answer.items);
    setSelectedId((current) => {
      if (answer.items.some((item) => item.instanceId === current))
        return current;
      return answer.items.find((item) => item.canReceive)?.instanceId ?? "";
    });
  }, [assetId]);

  useEffect(() => {
    if (!account) {
      setInstances([]);
      return;
    }
    void read();
  }, [account, read]);

  const selected = instances.find((item) => item.instanceId === selectedId);
  const pending = isPending(selected?.delivery ?? null);

  useEffect(() => {
    if (!pending) {
      watched.current = 0;
      return;
    }
    if (watched.current >= WATCH_LIMIT) return;
    const timer = setTimeout(() => {
      watched.current += 1;
      void read();
    }, WATCH_INTERVAL_MS);
    return () => clearTimeout(timer);
  }, [pending, read]);

  if (!account || instances.length === 0) return null;
  const sendable = instances.filter((item) => item.canReceive);
  if (sendable.length === 0) return null;
  if (!selected) return null;

  async function act(path: string, method: string, body?: unknown) {
    setBusy(true);
    setFailure("");
    try {
      const response = await browserFetch(path, {
        method,
        credentials: "same-origin",
        headers: body ? { "Content-Type": "application/json" } : undefined,
        body: body ? JSON.stringify(body) : undefined,
      });
      if (!response.ok) {
        const answer: unknown = await response.json().catch(() => null);
        setFailure(
          typeof answer === "object" &&
            answer !== null &&
            "error" in answer &&
            typeof answer.error === "string"
            ? answer.error
            : "Illarin could not do that. Try again in a moment.",
        );
        return;
      }
      await read();
    } catch {
      setFailure(
        "We could not reach Illarin. Check your connection and try again.",
      );
    } finally {
      setBusy(false);
    }
  }

  const send = () =>
    act(`/api/v1/assets/${assetId}/deliveries`, "POST", {
      instanceId: selected.instanceId,
    });
  const discard = (delivery: Delivery) =>
    act(`/api/v1/deliveries/${delivery.id}`, "DELETE");

  return (
    <section className={styles.panel} aria-labelledby="send-heading">
      <h2 id="send-heading">Send to an application</h2>

      {sendable.length > 1 ? (
        <label className={styles.picker}>
          <span>Installation</span>
          <select
            value={selectedId}
            onChange={(event) => setSelectedId(event.target.value)}
            disabled={busy}
          >
            {sendable.map((item) => (
              <option key={item.instanceId} value={item.instanceId}>
                {item.applicationName} — {item.instanceName}
              </option>
            ))}
          </select>
        </label>
      ) : (
        <p className={styles.only}>
          {selected.applicationName} — {selected.instanceName}
        </p>
      )}

      <InstanceState instance={selected} />

      <div className={styles.actions}>
        <button
          className={styles.send}
          type="button"
          onClick={send}
          disabled={busy || pending}
        >
          {pending ? (
            <Clock size={15} aria-hidden="true" />
          ) : (
            <Send size={15} aria-hidden="true" />
          )}
          {sendLabel(selected, busy)}
        </button>
        {selected.delivery ? (
          <button
            className={styles.secondary}
            type="button"
            onClick={() => selected.delivery && discard(selected.delivery)}
            disabled={busy}
          >
            {pending ? "Cancel" : "Dismiss"}
          </button>
        ) : null}
      </div>

      {failure ? (
        <output className={styles.failure} aria-live="polite">
          {failure}
        </output>
      ) : null}
    </section>
  );
}

function sendLabel(instance: Instance, busy: boolean): string {
  if (busy) return "Working…";
  if (isPending(instance.delivery)) return "Waiting to be collected";
  if (instance.updateAvailable) return "Send the update";
  if (instance.installedGeneration !== null) return "Send again";
  return "Send";
}

function InstanceState({ instance }: { instance: Instance }) {
  const delivery = instance.delivery;
  if (delivery && delivery.state !== "failed") {
    return (
      <p className={styles.state}>
        <Upload size={14} aria-hidden="true" />
        Waiting for {instance.instanceName} to collect it.
      </p>
    );
  }
  if (delivery?.state === "failed") {
    return (
      <p className={`${styles.state} ${styles.stopped}`}>
        <CircleAlert size={14} aria-hidden="true" />
        {FAILURE_LINES[delivery.reason ?? ""] ??
          "This delivery did not arrive."}
      </p>
    );
  }
  if (instance.updateAvailable) {
    return (
      <p className={`${styles.state} ${styles.behind}`}>
        <RefreshCw size={14} aria-hidden="true" />
        Installed, and a newer version exists here.
      </p>
    );
  }
  if (instance.installedGeneration !== null) {
    return (
      <p className={`${styles.state} ${styles.current}`}>
        <Check size={14} aria-hidden="true" />
        Installed and up to date.
      </p>
    );
  }
  if (!instance.reportsLibrary) {
    return (
      <p className={styles.state}>
        This installation does not report what it holds, so Illarin cannot say
        whether you already have it.
      </p>
    );
  }
  return <p className={styles.state}>Not installed here yet.</p>;
}
