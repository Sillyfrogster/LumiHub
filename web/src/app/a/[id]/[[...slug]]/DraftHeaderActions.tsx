"use client";

import { ArrowRight } from "lucide-react";
import { openCreatorMenu } from "./creator-menu";
import styles from "./DraftHeaderActions.module.css";

export function DraftHeaderActions() {
  return (
    <section className={styles.panel} aria-labelledby="draft-heading">
      <div>
        <h2 id="draft-heading">Your draft</h2>
        <p>Only you can open this page. It is not in browse or search.</p>
      </div>
      <button type="button" onClick={openCreatorMenu}>
        Publish
        <ArrowRight size={16} aria-hidden="true" />
      </button>
    </section>
  );
}
