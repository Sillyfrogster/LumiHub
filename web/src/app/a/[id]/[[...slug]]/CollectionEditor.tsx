"use client";

import { Plus, Trash2 } from "lucide-react";
import { type ReactNode, useId, useMemo, useRef, useState } from "react";
import styles from "./CollectionEditor.module.css";

/**
 * One row in the list beside the editor. `search` is everything a search
 * matches against, so an item is found by whatever a creator remembers of it.
 */
export type CollectionRow = {
  name: string;
  detail?: string;
  off?: boolean;
  search: string;
};

/**
 * A collection as a list beside an editor. A book of a thousand entries or a
 * preset of 338 fragments is read by scanning the list and edited one item at
 * a time, which is why this is not a stack of a thousand forms.
 */
export function CollectionEditor({
  rows,
  noun,
  emptyMessage,
  pending,
  selected,
  onSelect,
  onAdd,
  children,
}: {
  rows: CollectionRow[];
  noun: string;
  emptyMessage: string;
  pending: boolean;
  selected: number;
  onSelect: (index: number) => void;
  onAdd: () => void;
  children: ReactNode;
}) {
  const [search, setSearch] = useState("");
  const searchId = useId();
  const editor = useRef<HTMLDivElement>(null);

  const wanted = search.trim().toLowerCase();
  const matching = useMemo(
    () =>
      rows
        .map((row, index) => ({ row, index }))
        .filter(({ row }) => wanted === "" || row.search.includes(wanted)),
    [rows, wanted],
  );

  function select(index: number) {
    onSelect(index);
    // The panes stack at narrow width, so the editor needs bringing into view.
    editor.current?.scrollIntoView({ block: "nearest" });
  }

  return (
    <div className={styles.editor}>
      <div className={styles.list}>
        <div className={styles.search}>
          <label className={styles.hiddenLabel} htmlFor={searchId}>
            Search the {noun}s
          </label>
          <input
            id={searchId}
            type="search"
            value={search}
            placeholder={`Search the ${noun}s`}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
        {matching.length === 0 ? (
          <p className={styles.nothing}>
            {rows.length === 0 ? emptyMessage : `No ${noun} matches that.`}
          </p>
        ) : (
          <ol className={styles.entries}>
            {matching.map(({ row, index }) => (
              <li key={index}>
                <button
                  type="button"
                  aria-current={index === selected}
                  onClick={() => select(index)}
                >
                  <span className={styles.entryName}>{row.name}</span>
                  {row.detail ? (
                    <span className={styles.entryKeys}>{row.detail}</span>
                  ) : null}
                  {row.off ? (
                    <span className={styles.off}>Switched off</span>
                  ) : null}
                </button>
              </li>
            ))}
          </ol>
        )}
        <button
          type="button"
          className={styles.addEntry}
          onClick={() => {
            onAdd();
            select(rows.length);
          }}
          disabled={pending}
        >
          <Plus size={16} aria-hidden="true" />
          Add {noun}
        </button>
      </div>

      <div className={styles.detail} ref={editor}>
        {children}
      </div>
    </div>
  );
}

/** The heading over one item's fields, with the way to remove that item. */
export function ItemHeading({
  name,
  noun,
  pending,
  onRemove,
}: {
  name: string;
  noun: string;
  pending: boolean;
  onRemove: () => void;
}) {
  return (
    <div className={styles.detailHeading}>
      <h4>{name}</h4>
      <button
        type="button"
        className={styles.removeEntry}
        onClick={onRemove}
        disabled={pending}
      >
        <Trash2 size={14} aria-hidden="true" />
        Remove {noun}
      </button>
    </div>
  );
}

/** The stack one item's fields sit in. */
export function ItemFields({ children }: { children: ReactNode }) {
  return <div className={styles.fields}>{children}</div>;
}

/** Nothing chosen, or nothing there to choose. */
export function NothingChosen({ children }: { children: ReactNode }) {
  return <p className={styles.nothing}>{children}</p>;
}

/** One labelled control. */
export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: The control is the child, and the rule cannot see through the boundary.
    <label className={styles.field}>
      <span>
        {label}
        {hint ? <small> {hint}</small> : null}
      </span>
      {children}
    </label>
  );
}

/** A named set of related fields. */
export function FieldGroup({
  legend,
  children,
}: {
  legend: string;
  children: ReactNode;
}) {
  return (
    <fieldset className={styles.group}>
      <legend>{legend}</legend>
      {children}
    </fieldset>
  );
}

/** Two fields sharing a row until the surface is too narrow for them. */
export function FieldPair({ children }: { children: ReactNode }) {
  return <div className={styles.pair}>{children}</div>;
}

/** A yes or no, with the line that says what saying yes does. */
export function Switch({
  label,
  hint,
  checked,
  pending,
  onChange,
}: {
  label: string;
  hint?: string;
  checked: boolean;
  pending: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className={styles.switch}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        disabled={pending}
      />
      <span>
        {label}
        {hint ? <small>{hint}</small> : null}
      </span>
    </label>
  );
}

/** One list of short strings, one per line, kept in the order it is written. */
export function readLines(written: string): string[] {
  return written
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

export function writeLines(lines: string[] | undefined): string {
  return (lines ?? []).join("\n");
}

/** Returns the list with one item's fields changed, leaving the rest alone. */
export function replaceAt<T>(
  items: T[],
  index: number,
  changes: Partial<T>,
): T[] {
  return items.map((item, position) =>
    position === index ? { ...item, ...changes } : item,
  );
}

export function without<T>(items: T[], index: number): T[] {
  return items.filter((_, position) => position !== index);
}
