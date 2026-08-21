"use client";

import { ChevronUp } from "lucide-react";
import { useEffect, useRef } from "react";
import { FormattingNotice } from "@/components/ui/RichText";
import type { AssetElement, AssetImage } from "@/lib/api/query";
import { formattingWasRemoved, richTextsOf } from "@/lib/rich-text";
import { ElementContent } from "./ElementBody";
import elementStyles from "./ElementBody.module.css";
import styles from "./ElementReader.module.css";

export function ElementReader({
  element,
  images,
  onDismiss,
}: {
  element: AssetElement;
  images: AssetImage[];
  onDismiss: () => void;
}) {
  const dismiss = useRef<HTMLButtonElement>(null);
  const titleId = `element-reader-${element.id}`;

  useEffect(() => {
    dismiss.current?.focus({ preventScroll: true });
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      onDismiss();
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onDismiss]);

  return (
    <section className={styles.reader} aria-labelledby={titleId}>
      <header className={styles.header}>
        <h3 id={titleId}>{element.label || "Content"}</h3>
        <button ref={dismiss} type="button" onClick={onDismiss}>
          <ChevronUp size={16} aria-hidden="true" />
          Collapse
        </button>
      </header>
      <div className={styles.scroller}>
        <div className={elementStyles.element}>
          <ElementContent element={element} images={images} />
          {formattingWasRemoved(richTextsOf(element)) ? (
            <FormattingNotice />
          ) : null}
        </div>
      </div>
      <footer className={styles.footer}>Esc closes this reader.</footer>
    </section>
  );
}
