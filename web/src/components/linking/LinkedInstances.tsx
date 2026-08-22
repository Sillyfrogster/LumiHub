"use client";

import { Plug, PlugZap, RotateCcw } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import type { components } from "@/lib/api/schema";
import { useAuth } from "@/lib/auth";
import { readableDate } from "@/lib/dates";
import { describeScope } from "@/lib/scopes";
import styles from "./LinkedInstances.module.css";

type Instance = components["schemas"]["LinkedInstance"];
type Notice = { kind: "success" | "error"; message: string };

const BROWSER_MUTATION_HEADER = { "X-Illarin-Request": "1" } as const;

export function LinkedInstances() {
  const { account } = useAuth();
  const [instances, setInstances] = useState<Instance[] | null | undefined>(
    undefined,
  );
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [revoking, setRevoking] = useState("");
  const [confirming, setConfirming] = useState("");

  const load = useCallback(async () => {
    setLoadError("");
    setInstances(undefined);
    try {
      const response = await fetch("/api/v1/instances", {
        cache: "no-store",
        credentials: "same-origin",
      });
      const answer: unknown = await readJSON(response);
      if (!response.ok) {
        setLoadError(
          errorMessage(answer, "We could not read your linked instances."),
        );
        setInstances(null);
        return;
      }
      if (!isInstanceList(answer)) {
        setLoadError("Illarin returned an incomplete instance list.");
        setInstances(null);
        return;
      }
      setInstances(answer.items);
    } catch {
      setLoadError(
        "We could not reach Illarin. Check your connection and try again.",
      );
      setInstances(null);
    }
  }, []);

  useEffect(() => {
    if (account === undefined) return;
    if (!account) {
      setInstances([]);
      return;
    }
    void load();
  }, [account, load]);

  if (!account) return null;

  async function revoke(instance: Instance) {
    if (revoking) return;
    setNotice(null);
    setRevoking(instance.id);
    try {
      const response = await fetch(`/api/v1/instances/${instance.id}`, {
        method: "DELETE",
        credentials: "same-origin",
        headers: BROWSER_MUTATION_HEADER,
      });
      const answer: unknown = await readJSON(response);
      if (!response.ok) {
        setNotice({
          kind: "error",
          message: errorMessage(
            answer,
            `${instance.applicationName} — ${instance.instanceName} could not be revoked.`,
          ),
        });
        return;
      }

      const revokedAt = new Date().toISOString();
      setInstances(
        (current) =>
          current?.map((item) =>
            item.id === instance.id
              ? {
                  ...item,
                  applicationVersion: null,
                  protocolVersion: null,
                  capabilities: [],
                  acceptedTargets: [],
                  revokedAt,
                }
              : item,
          ) ?? current,
      );
      setConfirming("");
      setNotice({
        kind: "success",
        message: `${instance.applicationName} — ${instance.instanceName} can no longer reach your account. Other linked instances were not changed.`,
      });
    } catch {
      setNotice({
        kind: "error",
        message:
          "We could not reach Illarin. Check your connection and try again.",
      });
    } finally {
      setRevoking("");
    }
  }

  return (
    <section className={styles.instances}>
      <header className={styles.heading}>
        <div>
          <h2>Linked applications</h2>
          <p>
            Each entry is one independent installation with short-lived access
            tokens and a rotating refresh credential. Revoking one does not
            affect your other links.
          </p>
        </div>
        <Link className={styles.action} href="/link">
          Link an application
        </Link>
      </header>

      {notice ? (
        <output
          className={`${styles.notice} ${
            notice.kind === "error" ? styles.noticeError : styles.noticeSuccess
          }`}
          aria-live="polite"
        >
          {notice.message}
        </output>
      ) : null}

      <div className={styles.panel} aria-busy={instances === undefined}>
        {instances === undefined ? (
          <p className={styles.waiting} aria-live="polite">
            Reading your linked applications…
          </p>
        ) : instances === null ? (
          <div className={styles.loadFailure}>
            <RotateCcw size={21} strokeWidth={1.5} aria-hidden="true" />
            <div>
              <p>{loadError}</p>
              <button className={styles.retry} type="button" onClick={load}>
                Try again
              </button>
            </div>
          </div>
        ) : instances.length === 0 ? (
          <div className={styles.empty}>
            <Plug size={22} strokeWidth={1.4} aria-hidden="true" />
            <p>
              Nothing is linked yet. Open linking from an application, or enter
              the device code it gives you.
            </p>
          </div>
        ) : (
          <ul className={styles.list}>
            {instances.map((instance) => {
              const isRevoked = Boolean(instance.revokedAt);
              const isConfirming = confirming === instance.id;
              const isRevoking = revoking === instance.id;
              const confirmationId = `revoke-${instance.id}`;
              return (
                <li
                  key={instance.id}
                  className={isRevoked ? styles.revoked : undefined}
                >
                  <span className={styles.mark}>
                    {isRevoked ? (
                      <PlugZap size={20} strokeWidth={1.4} aria-hidden="true" />
                    ) : (
                      <Plug size={20} strokeWidth={1.4} aria-hidden="true" />
                    )}
                  </span>
                  <div className={styles.about}>
                    <h3>{instance.applicationName}</h3>
                    <p className={styles.instanceName}>
                      {instance.instanceName}
                    </p>
                    <p className={styles.prefix}>
                      <span>
                        {isRevoked ? "Last credential" : "Refresh credential"}
                      </span>
                      <code>{instance.prefix}</code>
                    </p>
                    <p className={styles.when}>
                      {instance.revokedAt
                        ? `Linked ${readableDate(instance.linkedAt)}, revoked ${readableDate(instance.revokedAt)}`
                        : `Linked ${readableDate(instance.linkedAt)} · ${
                            instance.lastSeenAt
                              ? `last seen ${readableDate(instance.lastSeenAt)}`
                              : "not seen yet"
                          }`}
                    </p>
                    <ScopeList scopes={instance.scopes} />
                    {!isRevoked ? (
                      <InstanceCompatibility instance={instance} />
                    ) : null}
                  </div>
                  {isRevoked ? (
                    <span className={styles.cut}>Revoked</span>
                  ) : (
                    <div className={styles.revokeActions}>
                      {isConfirming ? (
                        <p className={styles.confirmation} id={confirmationId}>
                          This cuts off only this installation.
                        </p>
                      ) : null}
                      <div className={styles.revokeButtons}>
                        <button
                          className={styles.action}
                          type="button"
                          onClick={() => {
                            if (isConfirming) {
                              void revoke(instance);
                            } else {
                              setConfirming(instance.id);
                            }
                          }}
                          disabled={Boolean(revoking)}
                          aria-describedby={
                            isConfirming ? confirmationId : undefined
                          }
                          aria-expanded={isConfirming}
                        >
                          {isRevoking
                            ? "Revoking…"
                            : isConfirming
                              ? "Confirm revoke"
                              : "Revoke"}
                        </button>
                        {isConfirming ? (
                          <button
                            className={styles.cancel}
                            type="button"
                            onClick={() => setConfirming("")}
                            disabled={Boolean(revoking)}
                          >
                            Cancel
                          </button>
                        ) : null}
                      </div>
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </section>
  );
}

function ScopeList({ scopes }: { scopes: Instance["scopes"] }) {
  return (
    <ul className={styles.scopes} aria-label="Granted permissions">
      {scopes.map((scope) => {
        const copy = describeScope(scope);
        return (
          <li key={scope} title={copy.detail}>
            {copy.title}
          </li>
        );
      })}
    </ul>
  );
}

function InstanceCompatibility({ instance }: { instance: Instance }) {
  return (
    <div className={styles.interoperability}>
      <p className={styles.interoperabilityNote}>
        Self-reported compatibility, not permission
      </p>
      <dl>
        {instance.applicationVersion ? (
          <div>
            <dt>Version</dt>
            <dd>{instance.applicationVersion}</dd>
          </div>
        ) : null}
        <div>
          <dt>Targets</dt>
          <dd>
            <InlineValues values={instance.acceptedTargets} />
          </dd>
        </div>
        <div>
          <dt>Capabilities</dt>
          <dd>
            <InlineValues values={instance.capabilities} />
          </dd>
        </div>
        {instance.protocolVersion !== null ? (
          <div>
            <dt>Protocol</dt>
            <dd>Version {instance.protocolVersion}</dd>
          </div>
        ) : null}
      </dl>
    </div>
  );
}

function InlineValues({ values }: { values: string[] }) {
  if (values.length === 0) {
    return <span className={styles.notDeclared}>None declared</span>;
  }
  return (
    <ul className={styles.values}>
      {values.map((value) => (
        <li key={value}>{value}</li>
      ))}
    </ul>
  );
}

async function readJSON(response: Response): Promise<unknown> {
  if (response.status === 204) return null;
  try {
    return await response.json();
  } catch {
    return null;
  }
}

function errorMessage(value: unknown, fallback: string) {
  if (
    typeof value === "object" &&
    value !== null &&
    "error" in value &&
    typeof value.error === "string" &&
    value.error.trim()
  ) {
    return value.error;
  }
  return fallback;
}

function isInstanceList(value: unknown): value is { items: Instance[] } {
  return (
    typeof value === "object" &&
    value !== null &&
    "items" in value &&
    Array.isArray(value.items) &&
    value.items.every(isInstance)
  );
}

function isInstance(value: unknown): value is Instance {
  if (typeof value !== "object" || value === null) return false;
  const instance = value as Record<string, unknown>;
  return (
    typeof instance.id === "string" &&
    typeof instance.applicationName === "string" &&
    typeof instance.instanceName === "string" &&
    (instance.applicationVersion === undefined ||
      instance.applicationVersion === null ||
      typeof instance.applicationVersion === "string") &&
    (instance.protocolVersion === null ||
      typeof instance.protocolVersion === "number") &&
    isStringArray(instance.capabilities) &&
    isStringArray(instance.acceptedTargets) &&
    typeof instance.prefix === "string" &&
    isStringArray(instance.scopes) &&
    typeof instance.linkedAt === "string" &&
    (instance.lastSeenAt === null || typeof instance.lastSeenAt === "string") &&
    (instance.revokedAt === null || typeof instance.revokedAt === "string")
  );
}

function isStringArray(value: unknown): value is string[] {
  return (
    Array.isArray(value) && value.every((item) => typeof item === "string")
  );
}
