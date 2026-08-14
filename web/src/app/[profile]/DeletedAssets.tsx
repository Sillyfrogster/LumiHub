"use client";

import { RotateCcw } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Shell } from "@/components/layout/Shell";
import type { DeletedAsset } from "@/lib/api/query";
import { restoreAsset } from "@/lib/api/query";
import { remainingDeletionWindow } from "@/lib/deletion-window";
import { KIND_LABELS } from "@/lib/kinds";
import styles from "./DeletedAssets.module.css";

function restoreDeadline(value: string) {
  return new Date(value).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

export function DeletedAssets({
  initialItems,
}: {
  initialItems: DeletedAsset[];
}) {
  const router = useRouter();
  const [items, setItems] = useState(initialItems);
  const [pending, setPending] = useState<string | null>(null);
  const [message, setMessage] = useState("");

  async function restore(item: DeletedAsset) {
    if (pending) return;
    setPending(item.id);
    setMessage("");
    setItems((current) =>
      current.filter((candidate) => candidate.id !== item.id),
    );
    try {
      await restoreAsset(item.id);
      router.refresh();
    } catch {
      setItems((current) => [item, ...current]);
      setMessage(`${item.name} could not be restored. Try again.`);
    } finally {
      setPending(null);
    }
  }

  return (
    <section
      id="deleted"
      className={styles.section}
      aria-labelledby="deleted-heading"
    >
      <Shell>
        <div className={styles.heading}>
          <div>
            <h2 id="deleted-heading">Deleted</h2>
            <p>
              These creations stay here briefly before their files are cleared.
            </p>
          </div>
          <RotateCcw size={21} aria-hidden="true" />
        </div>

        {message ? (
          <p className={styles.error} role="alert">
            {message}
          </p>
        ) : null}

        {items.length > 0 ? (
          <ul className={styles.list}>
            {items.map((item) => (
              <li key={item.id}>
                <div className={styles.identity}>
                  <span>{KIND_LABELS[item.kind]}</span>
                  <h3>{item.name}</h3>
                </div>
                <p>
                  <span suppressHydrationWarning>
                    {remainingDeletionWindow(item.recoverableUntil)}
                  </span>{" "}
                  · Restorable until{" "}
                  <time dateTime={item.recoverableUntil}>
                    {restoreDeadline(item.recoverableUntil)}
                  </time>
                </p>
                <button
                  type="button"
                  onClick={() => restore(item)}
                  disabled={pending !== null}
                >
                  <RotateCcw size={15} aria-hidden="true" />
                  {pending === item.id ? "Restoring…" : "Restore"}
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <p className={styles.empty}>Nothing is waiting to be restored.</p>
        )}
      </Shell>
    </section>
  );
}
