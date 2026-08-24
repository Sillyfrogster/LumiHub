"use client";

import { Lock } from "lucide-react";
import styles from "./SealedPanel.module.css";

/** Offers an owner the sealed content preserved during migration. */
export function SealedPanel({
  assetId,
  count,
}: {
  assetId: string;
  count: number;
}) {
  return (
    <section className={styles.panel} aria-labelledby="sealed-heading">
      <h2 id="sealed-heading">Your sealed content</h2>
      <p className={styles.lead}>
        {count === 1
          ? "One block of this preset was sealed on LumiHub"
          : `${count} blocks of this preset were sealed on LumiHub`}
        . Readers never saw them and still do not. Illarin has no way to put
        them back into a download, so take a copy and keep it.
      </p>
      <a className={styles.take} href={`/api/v1/assets/${assetId}/sealed`}>
        <Lock size={16} aria-hidden="true" />
        Download the sealed blocks
      </a>
    </section>
  );
}
