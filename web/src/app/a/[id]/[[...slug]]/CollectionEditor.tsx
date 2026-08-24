"use client";

import { Plus, Trash2 } from "lucide-react";
import { type ReactNode, useId, useMemo, useRef, useState } from "react";
import styles from "./CollectionEditor.module.css";

export type CollectionRow = {
  name: string;
  detail?: string;
  off?: boolean;
  sealed?: boolean;
  search: string;
};

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
                  {row.off || row.sealed ? (
                    <span className={styles.badges}>
                      {row.off ? (
                        <span className={styles.off}>Switched off</span>
                      ) : null}
                      {row.sealed ? (
                        <span className={styles.sealed}>Sealed</span>
                      ) : null}
                    </span>
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

export function ItemFields({ children }: { children: ReactNode }) {
  return <div className={styles.fields}>{children}</div>;
}

export function NothingChosen({ children }: { children: ReactNode }) {
  return <p className={styles.nothing}>{children}</p>;
}

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

export function FieldPair({ children }: { children: ReactNode }) {
  return <div className={styles.pair}>{children}</div>;
}

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

export function readLines(written: string): string[] {
  return written
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

export function writeLines(lines: string[] | undefined): string {
  return (lines ?? []).join("\n");
}

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
