"use client";

import { Check, ChevronDown, Columns3, LayoutGrid } from "lucide-react";
import { useRef } from "react";
import type { AssetBlock } from "@/lib/api/query";
import {
  type BlockLayout,
  type BlockWidth,
  LAYOUT_LABELS,
  LAYOUTS,
  layoutChoiceIssue,
  WIDTH_COLUMNS,
  WIDTH_LABELS,
  widthChoiceIssue,
} from "@/lib/page-arrangement";
import styles from "./ArrangementPickers.module.css";

const WIDTHS = ["full", "two_thirds", "half", "third"] as const;
const BARS = Array.from({ length: 12 }, (_, index) => index + 1);

const WIDTH_HINTS: Record<BlockWidth, string> = {
  full: "One section across the row",
  two_thirds: "Leaves room for a shorter third",
  half: "Pairs evenly with another half",
  third: "For short supporting sections",
};

const LAYOUT_HINTS: Record<BlockLayout, string> = {
  single: "One content area",
  duo: "Two equal areas side by side",
  "main-aside": "One wide area and one narrow area",
  trio: "Three equal areas side by side",
  "stack-2": "Two content areas in reading order",
  "stack-3": "Three content areas in reading order",
};

export function WidthPicker({
  width,
  layout,
  pending = false,
  inline = false,
  onSelect,
  onIssue,
}: {
  width: AssetBlock["width"];
  layout: AssetBlock["layout"];
  pending?: boolean;
  inline?: boolean;
  onSelect: (width: AssetBlock["width"]) => void;
  onIssue: (message: string) => void;
}) {
  const disclosure = useRef<HTMLDetailsElement>(null);

  return (
    <details
      className={`${styles.picker} ${styles.widthPicker} ${inline ? styles.inline : ""}`}
      ref={disclosure}
    >
      <summary
        className={styles.trigger}
        aria-label={`Change section width. Current width: ${WIDTH_LABELS[width]}`}
        aria-disabled={pending}
      >
        <Columns3 size={15} aria-hidden="true" />
        <WidthBars width={width} />
        <span className={styles.triggerLabel}>{WIDTH_LABELS[width]}</span>
        <ChevronDown className={styles.chevron} size={14} aria-hidden="true" />
      </summary>
      <div className={styles.menu}>
        <div className={styles.menuHeading}>
          <strong>Section width</strong>
          <span>A section on its own row fills the page.</span>
        </div>
        <div className={styles.options}>
          {WIDTHS.map((choice) => {
            const issue = widthChoiceIssue(layout, choice);
            return (
              <button
                type="button"
                className={styles.option}
                key={choice}
                aria-pressed={choice === width}
                disabled={pending}
                onClick={() => {
                  if (issue) {
                    onIssue(issue);
                    return;
                  }
                  onIssue("");
                  onSelect(choice);
                  closeDisclosure(disclosure.current);
                }}
              >
                <WidthBars width={choice} />
                <span className={styles.optionCopy}>
                  <strong>{WIDTH_LABELS[choice]}</strong>
                  <small>{issue ?? WIDTH_HINTS[choice]}</small>
                </span>
                {choice === width ? (
                  <Check
                    className={styles.check}
                    size={16}
                    aria-hidden="true"
                  />
                ) : null}
              </button>
            );
          })}
        </div>
      </div>
    </details>
  );
}

export function LayoutPicker({
  layout,
  width,
  allowedLayouts,
  elementLabels,
  pending = false,
  inline = false,
  onSelect,
  onIssue,
}: {
  layout: AssetBlock["layout"];
  width: AssetBlock["width"];
  allowedLayouts: AssetBlock["allowedLayouts"];
  elementLabels: string[];
  pending?: boolean;
  inline?: boolean;
  onSelect: (layout: AssetBlock["layout"]) => void;
  onIssue: (message: string) => void;
}) {
  const disclosure = useRef<HTMLDetailsElement>(null);

  return (
    <details
      className={`${styles.picker} ${inline ? styles.inline : ""}`}
      ref={disclosure}
    >
      <summary
        className={styles.trigger}
        aria-label={`Change section layout. Current layout: ${LAYOUT_LABELS[layout]}`}
        aria-disabled={pending}
      >
        <LayoutGrid size={15} aria-hidden="true" />
        <span className={styles.triggerLabel}>{LAYOUT_LABELS[layout]}</span>
        <ChevronDown className={styles.chevron} size={14} aria-hidden="true" />
      </summary>
      <div className={`${styles.menu} ${styles.layoutMenu}`}>
        <div className={styles.menuHeading}>
          <strong>Content layout</strong>
          <span>Every element keeps its place in reading order.</span>
        </div>
        <div className={styles.options}>
          {allowedLayouts.map((choice) => {
            const issue = layoutChoiceIssue(choice, width, elementLabels);
            return (
              <button
                type="button"
                className={styles.option}
                key={choice}
                aria-pressed={choice === layout}
                disabled={pending}
                onClick={() => {
                  if (issue) {
                    onIssue(issue);
                    return;
                  }
                  onIssue("");
                  onSelect(choice);
                  closeDisclosure(disclosure.current);
                }}
              >
                <LayoutGlyph layout={choice} />
                <span className={styles.optionCopy}>
                  <strong>{LAYOUT_LABELS[choice]}</strong>
                  <small>{issue ?? LAYOUT_HINTS[choice]}</small>
                </span>
                {choice === layout ? (
                  <Check
                    className={styles.check}
                    size={16}
                    aria-hidden="true"
                  />
                ) : null}
              </button>
            );
          })}
        </div>
      </div>
    </details>
  );
}

function closeDisclosure(disclosure: HTMLDetailsElement | null) {
  disclosure?.removeAttribute("open");
  disclosure?.querySelector("summary")?.focus();
}

function WidthBars({ width }: { width: BlockWidth }) {
  const filled = WIDTH_COLUMNS[width];
  return (
    <span className={styles.widthBars} aria-hidden="true">
      {BARS.map((bar) => (
        <span
          className={bar <= filled ? styles.filledBar : undefined}
          key={bar}
        />
      ))}
    </span>
  );
}

function LayoutGlyph({ layout }: { layout: BlockLayout }) {
  return (
    <span
      className={`${styles.layoutGlyph} ${styles[layout.replace("-", "")]}`}
      aria-hidden="true"
    >
      {LAYOUTS[layout].slots.map((slot) => (
        <span key={slot} />
      ))}
    </span>
  );
}
