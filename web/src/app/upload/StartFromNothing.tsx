"use client";

import { PenLine } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { startAsset } from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";
import { useAuth } from "@/lib/auth";
import { BUILDABLE_KINDS, KIND_LABELS } from "@/lib/kinds";
import styles from "./StartFromNothing.module.css";

/**
 * The other way in. A creator picks the kind and lands on a page already
 * carrying the sections that kind requires.
 */
export function StartFromNothing() {
  const router = useRouter();
  const { account } = useAuth();
  const [pending, setPending] = useState("");
  const [message, setMessage] = useState("");

  if (!account?.emailVerified) return null;

  async function start(kind: string) {
    setPending(kind);
    setMessage("");
    try {
      const started = await startAsset(kind);
      router.push(assetHref(started.id, started.name));
    } catch {
      setPending("");
      setMessage("The draft could not be started. Try again.");
    }
  }

  return (
    <section className={styles.panel} aria-labelledby="start-heading">
      <h2 id="start-heading">Or start from nothing</h2>
      <p>
        Pick what you are making and get a page with the sections it needs,
        empty and waiting. It stays a draft that only you can open until you
        publish it.
      </p>
      <div className={styles.kinds}>
        {BUILDABLE_KINDS.map((kind) => (
          <button
            key={kind}
            type="button"
            onClick={() => start(kind)}
            disabled={pending !== ""}
          >
            <PenLine size={16} aria-hidden="true" />
            {pending === kind
              ? "Starting…"
              : `Start a ${KIND_LABELS[kind].toLowerCase()}`}
          </button>
        ))}
      </div>
      {message ? (
        <p className={styles.error} role="alert">
          {message}
        </p>
      ) : null}
    </section>
  );
}
