"use client";

import { Plug, PlugZap } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import type { components } from "@/lib/api/schema";
import { useAuth } from "@/lib/auth";
import { readableDate } from "@/lib/dates";
import { describeScope } from "@/lib/scopes";
import styles from "./LinkedInstances.module.css";

type Instance = components["schemas"]["LinkedInstance"];

export function LinkedInstances() {
  const { account } = useAuth();
  const [instances, setInstances] = useState<Instance[] | undefined>(undefined);
  const [notice, setNotice] = useState("");
  const [revoking, setRevoking] = useState("");

  const load = useCallback(async () => {
    try {
      const response = await fetch("/api/v1/instances", {
        cache: "no-store",
        credentials: "same-origin",
      });
      if (!response.ok) {
        setInstances([]);
        return;
      }
      const answer = (await response.json()) as { items: Instance[] };
      setInstances(answer.items);
    } catch {
      setInstances([]);
      setNotice("We could not read your linked instances.");
    }
  }, []);

  useEffect(() => {
    if (!account) {
      setInstances([]);
      return;
    }
    void load();
  }, [account, load]);

  if (!account) return null;

  async function revoke(instance: Instance) {
    setNotice("");
    setRevoking(instance.id);
    try {
      const response = await fetch(`/api/v1/instances/${instance.id}`, {
        method: "DELETE",
        credentials: "same-origin",
      });
      if (!response.ok) {
        setNotice(`${instance.name} could not be revoked.`);
        return;
      }
      await load();
      setNotice(`${instance.name} can no longer reach your account.`);
    } catch {
      setNotice(
        "We could not reach LumiHub. Check your connection and try again.",
      );
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
            Each entry is one installation you approved. Revoking cuts it off at
            once and throws away everything it was holding here.
          </p>
        </div>
        <Link className={styles.action} href="/link">
          Link an application
        </Link>
      </header>

      {notice ? (
        <output className={styles.notice} aria-live="polite">
          {notice}
        </output>
      ) : null}

      <div className={styles.panel}>
        {instances === undefined ? (
          <p className={styles.waiting} aria-live="polite">
            Reading your linked applications…
          </p>
        ) : instances.length === 0 ? (
          <div className={styles.empty}>
            <Plug size={22} strokeWidth={1.4} aria-hidden="true" />
            <p>
              Nothing is linked yet. An application shows you a code, you
              approve it here, and it can then receive what you send it.
            </p>
          </div>
        ) : (
          <ul className={styles.list}>
            {instances.map((instance) => (
              <li
                key={instance.id}
                className={instance.revokedAt ? styles.revoked : undefined}
              >
                <span className={styles.mark}>
                  {instance.revokedAt ? (
                    <PlugZap size={20} strokeWidth={1.4} aria-hidden="true" />
                  ) : (
                    <Plug size={20} strokeWidth={1.4} aria-hidden="true" />
                  )}
                </span>
                <div className={styles.about}>
                  <h3>{instance.name}</h3>
                  <p className={styles.prefix}>{instance.prefix}</p>
                  <p className={styles.when}>
                    {instance.revokedAt
                      ? `Linked ${readableDate(instance.linkedAt)}, revoked ${readableDate(instance.revokedAt)}`
                      : `Linked ${readableDate(instance.linkedAt)} · ${
                          instance.lastSeenAt
                            ? `last seen ${readableDate(instance.lastSeenAt)}`
                            : "not seen yet"
                        }`}
                  </p>
                  <ul className={styles.scopes}>
                    {instance.scopes.map((scope) => (
                      <li key={scope} title={describeScope(scope).detail}>
                        {describeScope(scope).title}
                      </li>
                    ))}
                  </ul>
                </div>
                {instance.revokedAt ? (
                  <span className={styles.cut}>Revoked</span>
                ) : (
                  <button
                    className={styles.action}
                    type="button"
                    onClick={() => revoke(instance)}
                    disabled={revoking === instance.id}
                  >
                    {revoking === instance.id ? "Revoking…" : "Revoke"}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      <p className={styles.footnote}>
        Building something that links here? The{" "}
        <a href="/protocol">protocol guide</a> and the{" "}
        <a href="/openapi.yaml">OpenAPI file</a> are all you need. Nobody
        registers.
      </p>
    </section>
  );
}
