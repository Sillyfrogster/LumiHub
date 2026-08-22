"use client";

import { PenLine } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import {
  type BrowseKind,
  type StartAssetApp,
  startAsset,
} from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";
import { useAuth } from "@/lib/auth";
import {
  APP_CHOICES,
  BUILDABLE_KINDS,
  KIND_LABELS,
  KINDS_ASKING_FOR_AN_APP,
} from "@/lib/kinds";
import styles from "./StartFromNothing.module.css";

export function StartFromNothing() {
  const router = useRouter();
  const { account } = useAuth();
  const [pending, setPending] = useState("");
  const [asking, setAsking] = useState<BrowseKind | null>(null);
  const [message, setMessage] = useState("");

  if (!account?.emailVerified) return null;

  async function start(kind: BrowseKind, app?: StartAssetApp) {
    setPending(kind);
    setMessage("");
    try {
      const started = await startAsset(kind, app);
      router.push(assetHref(started.id, started.name));
    } catch {
      setPending("");
      setMessage("The draft could not be started. Try again.");
    }
  }

  function choose(kind: BrowseKind) {
    if (KINDS_ASKING_FOR_AN_APP.includes(kind)) {
      setMessage("");
      setAsking(asking === kind ? null : kind);
      return;
    }
    void start(kind);
  }

  return (
    <section className={styles.panel} aria-labelledby="start-heading">
      <h2 id="start-heading">Start a new asset</h2>
      <p>
        Choose a supported kind. It opens as a private draft and stays private
        until you publish it.
      </p>
      <div className={styles.kinds}>
        {BUILDABLE_KINDS.map((kind) => (
          <button
            key={kind}
            type="button"
            aria-expanded={
              KINDS_ASKING_FOR_AN_APP.includes(kind)
                ? asking === kind
                : undefined
            }
            onClick={() => choose(kind)}
            disabled={pending !== ""}
          >
            <PenLine size={16} aria-hidden="true" />
            {pending === kind
              ? "Starting…"
              : `Start a ${KIND_LABELS[kind].toLowerCase()}`}
          </button>
        ))}
      </div>
      {asking ? (
        <div className={styles.apps}>
          <p className={styles.appQuestion}>
            Which app is this {KIND_LABELS[asking].toLowerCase()} for? Its
            editable fields get that app's names. Nothing else about the{" "}
            {KIND_LABELS[asking].toLowerCase()} depends on the answer, and you
            are not asked again.
          </p>
          <div className={styles.appChoices}>
            {APP_CHOICES.map((app) => (
              <button
                key={app.value}
                type="button"
                onClick={() => void start(asking, app.value)}
                disabled={pending !== ""}
              >
                {app.label}
              </button>
            ))}
          </div>
        </div>
      ) : null}
      {message ? (
        <p className={styles.error} role="alert">
          {message}
        </p>
      ) : null}
    </section>
  );
}
