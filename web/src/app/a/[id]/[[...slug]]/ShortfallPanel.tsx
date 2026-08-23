"use client";

import { AlertCircle } from "lucide-react";
import type { ReadinessItem } from "@/lib/api/query";
import styles from "./PublishPanel.module.css";

/** Where a creator goes to fill one thing in. */
function itemHref(item: ReadinessItem): string {
  if (item.blockId) return `#block-${item.blockId}`;
  return item.id === "adult_content" ? "#adult-content-answer" : "#asset-name";
}

export function ShortfallPanel({
  kind,
  readiness,
  onNavigateToBlock,
}: {
  kind: string;
  readiness: ReadinessItem[];
  onNavigateToBlock: (blockId: string) => void;
}) {
  const missing = readiness.filter((item) => !item.met);
  if (missing.length === 0) return null;

  return (
    <section className={styles.panel} aria-labelledby="shortfall-heading">
      <h2 id="shortfall-heading">Worth filling in</h2>
      <p className={styles.privacy}>
        Your page is public and stays public. A new {kind} would be asked for
        {missing.length === 1 ? " this" : " these"} before it could be shared.
      </p>

      <ul className={styles.checklist}>
        {missing.map((item) => (
          <li key={item.id}>
            <span className={styles.mark} aria-hidden="true">
              <AlertCircle size={15} />
            </span>
            <span>
              <strong>{item.label}</strong>
              <span className={styles.detail}>{item.detail}</span>
              <a
                href={itemHref(item)}
                onClick={(event) => {
                  if (!item.blockId) return;
                  event.preventDefault();
                  onNavigateToBlock(item.blockId);
                }}
              >
                take me there
              </a>
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
