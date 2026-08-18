"use client";

import { AlertCircle, Check } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import type { ReadinessItem } from "@/lib/api/query";
import { publishAsset } from "@/lib/api/query";
import styles from "./PublishPanel.module.css";

/** Where a creator goes to meet one requirement. */
function itemHref(item: ReadinessItem): string {
  return item.blockId ? `#block-${item.blockId}` : "#asset-header";
}

export function PublishPanel({
  assetId,
  kind,
  readiness,
}: {
  assetId: string;
  kind: string;
  readiness: ReadinessItem[];
}) {
  const router = useRouter();
  const dialog = useRef<HTMLDialogElement>(null);
  const [items, setItems] = useState(readiness);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => setItems(readiness), [readiness]);

  const missing = items.filter((item) => !item.met);

  async function publish() {
    if (pending) return;
    setPending(true);
    setMessage("");
    const answer = await publishAsset(assetId);
    setPending(false);
    if (answer.published) {
      dialog.current?.close();
      router.refresh();
      return;
    }
    if (answer.readiness) {
      setItems(answer.readiness);
      return;
    }
    setMessage(answer.error);
  }

  return (
    <section className={styles.panel} aria-labelledby="publish-heading">
      <h2 id="publish-heading">Before you publish</h2>
      <p className={styles.privacy}>
        Only you can open this page. It is in no browse or search result, and it
        has nothing to download.
      </p>

      <ul className={styles.checklist}>
        {items.map((item) => (
          <li key={item.id} data-met={item.met || undefined}>
            <span className={styles.mark} aria-hidden="true">
              {item.met ? <Check size={15} /> : <AlertCircle size={15} />}
            </span>
            <span>
              <strong>{item.label}</strong>
              <span className={styles.detail}>{item.detail}</span>
              {item.met ? null : <a href={itemHref(item)}>take me there</a>}
            </span>
          </li>
        ))}
      </ul>

      <button
        type="button"
        className={styles.publish}
        onClick={() => dialog.current?.showModal()}
      >
        Publish
      </button>
      <p className={styles.note}>
        Publishing is one-way. A blurb is never required.
      </p>
      {message ? (
        <p className={styles.error} role="alert">
          {message}
        </p>
      ) : null}

      <dialog
        ref={dialog}
        className={styles.dialog}
        aria-labelledby="publish-dialog-heading"
      >
        <h2 id="publish-dialog-heading">
          {missing.length > 0 ? "Not quite ready" : `Publish this ${kind}?`}
        </h2>
        {missing.length > 0 ? (
          <>
            <p>
              {missing.length === 1
                ? "One thing is still missing:"
                : `${missing.length} things are still missing:`}
            </p>
            <ul className={styles.missing}>
              {missing.map((item) => (
                <li key={item.id}>
                  <strong>{item.label}</strong> {item.detail}{" "}
                  <a
                    href={itemHref(item)}
                    onClick={() => dialog.current?.close()}
                  >
                    go to it
                  </a>
                </li>
              ))}
            </ul>
            <p className={styles.note}>
              Your draft keeps saving either way. Nothing is lost by pressing
              this.
            </p>
          </>
        ) : (
          <p>
            This becomes a public page anyone can open. It is one-way, and a
            published {kind} never returns to a draft.
          </p>
        )}
        <div className={styles.actions}>
          {missing.length > 0 ? null : (
            <button
              type="button"
              className={styles.publish}
              onClick={publish}
              disabled={pending}
            >
              {pending ? "Publishing…" : "Publish"}
            </button>
          )}
          <button
            type="button"
            className={styles.cancel}
            onClick={() => dialog.current?.close()}
          >
            {missing.length > 0 ? "Back to the draft" : "Cancel"}
          </button>
        </div>
      </dialog>
    </section>
  );
}
