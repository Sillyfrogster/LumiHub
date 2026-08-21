"use client";

import { CornerUpLeft, X } from "lucide-react";
import { useEffect, useRef } from "react";
import type { AssetElement, AssetImage } from "@/lib/api/query";
import { ElementFields } from "./ElementEditors";
import styles from "./ElementOverlay.module.css";

/** The full-screen surface edits large collection elements. */
export function ElementOverlay({
  assetId,
  element,
  images,
  returnLabel,
  pending,
  message,
  onChange,
  onLeave,
  onImageAdded,
}: {
  assetId: string;
  element: AssetElement;
  images: AssetImage[];
  returnLabel: string;
  pending: boolean;
  message: string;
  onChange: (element: AssetElement) => void;
  onLeave: () => void;
  onImageAdded: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    dialog.current?.showModal();
  }, []);

  return (
    <dialog
      ref={dialog}
      className={styles.overlay}
      aria-labelledby="element-overlay-title"
      onCancel={(event) => {
        // Esc leaves the way the buttons do, rather than closing the dialog.
        event.preventDefault();
        if (!pending) onLeave();
      }}
    >
      <div className={styles.frame}>
        <header className={styles.header}>
          <div className={styles.heading}>
            <p className={styles.context}>Editing in full screen</p>
            <h2 id="element-overlay-title">{element.label || "Content"}</h2>
            {element.facts.length > 0 ? (
              <p className={styles.facts}>{element.facts.join(" · ")}</p>
            ) : null}
          </div>
          <div className={styles.exits}>
            <button
              type="button"
              className={styles.return}
              disabled={pending}
              onClick={onLeave}
            >
              <CornerUpLeft size={16} aria-hidden="true" />
              {returnLabel}
            </button>
            <button
              type="button"
              className={styles.close}
              aria-label="Close full screen editing"
              disabled={pending}
              onClick={onLeave}
            >
              <X size={20} aria-hidden="true" />
            </button>
          </div>
        </header>

        <div className={styles.scroller}>
          {message ? (
            <p className={styles.error} role="alert">
              {message}
            </p>
          ) : null}
          <ElementFields
            assetId={assetId}
            element={element}
            images={images}
            pending={pending}
            onChange={onChange}
            onImageAdded={onImageAdded}
          />
        </div>

        <footer className={styles.footer}>
          <p>Esc leaves too, and keeps what you have written.</p>
          <button
            type="button"
            className={styles.done}
            disabled={pending}
            onClick={onLeave}
          >
            {pending ? "Saving…" : "Done"}
          </button>
        </footer>
      </div>
    </dialog>
  );
}
