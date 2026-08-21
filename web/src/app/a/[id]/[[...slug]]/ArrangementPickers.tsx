"use client";

import { Check, ChevronDown, Columns3, LayoutGrid } from "lucide-react";
import { useRef } from "react";
import type { AssetBlock } from "@/lib/api/query";
import {
  BLOCK_WIDTHS,
  type BlockLayout,
  type BlockWidth,
  LAYOUT_LABELS,
  LAYOUTS,
  layoutChoiceIssue,
  WIDTH_FLOORS_PX,
  WIDTH_LABELS,
  widthChoiceIssue,
} from "@/lib/page-arrangement";
import styles from "./ArrangementPickers.module.css";

const WIDTH_HINTS: Record<BlockWidth, string> = {
  full: "Uses all twelve columns.",
  two_thirds: `Uses eight columns, or Full if that would be under ${WIDTH_FLOORS_PX.two_thirds}px.`,
  half: `Uses six columns, or Full if that would be under ${WIDTH_FLOORS_PX.half}px.`,
  third: `Uses four columns, then Half or Full before it falls under ${WIDTH_FLOORS_PX.third}px.`,
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
  suggestedWidth,
  onSelect,
  onIssue,
}: {
  width: AssetBlock["width"];
  layout: AssetBlock["layout"];
  pending?: boolean;
  inline?: boolean;
  suggestedWidth?: AssetBlock["width"] | null;
  onSelect: (width: AssetBlock["width"]) => void;
  onIssue: (message: string) => void;
}) {
  const disclosure = useRef<HTMLDetailsElement>(null);
  const suggestion =
    suggestedWidth &&
    suggestedWidth !== width &&
    !widthChoiceIssue(layout, suggestedWidth)
      ? suggestedWidth
      : null;

  return (
    <div className={styles.widthControl}>
      <details
        className={`${styles.picker} ${styles.widthPicker} ${inline ? styles.inline : ""}`}
        ref={disclosure}
      >
        <summary
          className={styles.trigger}
          aria-label={`Change section width. Current width: ${WIDTH_LABELS[width]}`}
        >
          <Columns3 size={15} aria-hidden="true" />
          <span className={styles.triggerLabel}>{WIDTH_LABELS[width]}</span>
          <ChevronDown
            className={styles.chevron}
            size={14}
            aria-hidden="true"
          />
        </summary>
        <div className={styles.menu}>
          <div className={styles.menuHeading}>
            <strong>Section width</strong>
            <span>Sizes stay exact. A short row keeps its empty space.</span>
          </div>
          <div className={styles.options}>
            {BLOCK_WIDTHS.map((choice) => {
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
      {suggestion ? (
        <button
          type="button"
          className={styles.suggestion}
          disabled={pending}
          onClick={() => {
            onIssue("");
            onSelect(suggestion);
          }}
        >
          Use suggested: {WIDTH_LABELS[suggestion]}
        </button>
      ) : null}
    </div>
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
