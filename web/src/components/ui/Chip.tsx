"use client";

import Link from "next/link";
import { useState } from "react";
import styles from "./Chip.module.css";

export type ChipItem = {
  /** Distinct within the set. A lorebook may list the same key word twice. */
  id: string;
  label: string;
  href?: string;
};

/**
 * A set of short values a reader scans rather than reads. Asset tags and
 * lorebook keys share it. A limit keeps a long set from growing the page it
 * sits in.
 */
export function ChipSet({
  items,
  limit,
  className,
}: {
  items: readonly ChipItem[];
  limit?: number;
  className?: string;
}) {
  const [showingAll, setShowingAll] = useState(false);
  const held = limit === undefined ? 0 : Math.max(items.length - limit, 0);
  const shown = held > 0 && !showingAll ? items.slice(0, limit) : items;

  return (
    <div className={className}>
      <ul className={styles.list}>
        {shown.map((item) => (
          <li key={item.id}>
            {item.href ? (
              <Link
                className={`${styles.chip} ${styles.link}`}
                href={item.href}
              >
                {item.label}
              </Link>
            ) : (
              <span className={styles.chip}>{item.label}</span>
            )}
          </li>
        ))}
        {held > 0 ? (
          <li>
            <button
              type="button"
              className={`${styles.chip} ${styles.more}`}
              aria-expanded={showingAll}
              onClick={() => setShowingAll((current) => !current)}
            >
              {showingAll ? "Fewer" : `${held} more`}
            </button>
          </li>
        ) : null}
      </ul>
    </div>
  );
}
