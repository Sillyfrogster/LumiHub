"use client";

import { Plus, Trash2 } from "lucide-react";
import { useId, useMemo, useRef, useState } from "react";
import type { LorebookEntry } from "@/lib/api/query";
import styles from "./EntryTableEditor.module.css";

/** One key per line, so a key holding a comma is still one key. */
function readKeys(written: string): string[] {
  return written
    .split("\n")
    .map((key) => key.trim())
    .filter((key) => key !== "");
}

function writeKeys(keys: string[] | undefined): string {
  return (keys ?? []).join("\n");
}

export function entryName(entry: LorebookEntry, position: number): string {
  if (entry.name?.trim()) return entry.name;
  if (entry.keys.length > 0) return entry.keys.join(", ");
  return `Entry ${position + 1}`;
}

/**
 * A lorebook as a list beside an editor. A book of a thousand entries is read
 * by scanning the list and edited one entry at a time, which is why this is
 * not a stack of a thousand forms.
 */
export function EntryTableEditor({
  entries,
  pending,
  onChange,
}: {
  entries: LorebookEntry[];
  pending: boolean;
  onChange: (entries: LorebookEntry[]) => void;
}) {
  const [selected, setSelected] = useState(0);
  const [search, setSearch] = useState("");
  const searchId = useId();
  const editor = useRef<HTMLDivElement>(null);

  const wanted = search.trim().toLowerCase();
  const matching = useMemo(
    () =>
      entries
        .map((entry, index) => ({ entry, index }))
        .filter(
          ({ entry, index }) =>
            wanted === "" ||
            entryName(entry, index).toLowerCase().includes(wanted) ||
            entry.text.toLowerCase().includes(wanted) ||
            entry.keys.some((key) => key.toLowerCase().includes(wanted)),
        ),
    [entries, wanted],
  );

  const current = entries[selected];

  function select(index: number) {
    setSelected(index);
    // The panes stack at narrow width, so the editor needs bringing into view.
    editor.current?.scrollIntoView({ block: "nearest" });
  }

  function replaceCurrent(changes: Partial<LorebookEntry>) {
    onChange(
      entries.map((entry, index) =>
        index === selected ? { ...entry, ...changes } : entry,
      ),
    );
  }

  function addEntry() {
    onChange([...entries, { keys: [], text: "", enabled: true }]);
    select(entries.length);
  }

  function removeCurrent() {
    onChange(entries.filter((_, index) => index !== selected));
    setSelected(Math.max(0, Math.min(selected, entries.length - 2)));
  }

  return (
    <div className={styles.editor}>
      <div className={styles.list}>
        <div className={styles.search}>
          <label className={styles.hiddenLabel} htmlFor={searchId}>
            Search the entries
          </label>
          <input
            id={searchId}
            type="search"
            value={search}
            placeholder="Search names, keys and text"
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
        {matching.length === 0 ? (
          <p className={styles.nothing}>
            {entries.length === 0
              ? "This book has no entries yet."
              : "No entry matches that."}
          </p>
        ) : (
          <ol className={styles.entries}>
            {matching.map(({ entry, index }) => (
              <li key={index}>
                <button
                  type="button"
                  aria-current={index === selected}
                  onClick={() => select(index)}
                >
                  <span className={styles.entryName}>
                    {entryName(entry, index)}
                  </span>
                  <span className={styles.entryKeys}>
                    {entry.keys.length === 0
                      ? "No keys"
                      : entry.keys.slice(0, 4).join(" · ")}
                  </span>
                  {entry.enabled ? null : (
                    <span className={styles.off}>Switched off</span>
                  )}
                </button>
              </li>
            ))}
          </ol>
        )}
        <button
          type="button"
          className={styles.addEntry}
          onClick={addEntry}
          disabled={pending}
        >
          <Plus size={16} aria-hidden="true" />
          Add entry
        </button>
      </div>

      <div className={styles.detail} ref={editor}>
        {current ? (
          <EntryFields
            entry={current}
            position={selected}
            pending={pending}
            onChange={replaceCurrent}
            onRemove={removeCurrent}
          />
        ) : (
          <p className={styles.nothing}>
            Choose an entry to edit it, or add the first one.
          </p>
        )}
      </div>
    </div>
  );
}

function EntryFields({
  entry,
  position,
  pending,
  onChange,
  onRemove,
}: {
  entry: LorebookEntry;
  position: number;
  pending: boolean;
  onChange: (changes: Partial<LorebookEntry>) => void;
  onRemove: () => void;
}) {
  const recursion = entry.recursion ?? {};
  return (
    <div className={styles.fields}>
      <div className={styles.detailHeading}>
        <h4>{entryName(entry, position)}</h4>
        <button
          type="button"
          className={styles.removeEntry}
          onClick={onRemove}
          disabled={pending}
        >
          <Trash2 size={14} aria-hidden="true" />
          Remove entry
        </button>
      </div>

      <label className={styles.field}>
        <span>
          Name <small>optional, and never sent to a model</small>
        </span>
        <input
          value={entry.name ?? ""}
          onChange={(event) =>
            onChange({ name: event.target.value || undefined })
          }
          disabled={pending}
        />
      </label>

      <label className={styles.field}>
        <span>Entry text</span>
        <textarea
          rows={8}
          value={entry.text}
          onChange={(event) => onChange({ text: event.target.value })}
          disabled={pending}
        />
      </label>

      <fieldset className={styles.group}>
        <legend>What switches it on</legend>
        <label className={styles.field}>
          <span>
            Keys <small>one per line</small>
          </span>
          <textarea
            rows={4}
            value={writeKeys(entry.keys)}
            onChange={(event) =>
              onChange({ keys: readKeys(event.target.value) })
            }
            disabled={pending}
          />
        </label>
        <Switch
          label="Switched on"
          hint="A switched-off entry stays in the book and reaches no model."
          checked={entry.enabled}
          pending={pending}
          onChange={(enabled) => onChange({ enabled })}
        />
        <Switch
          label="Always on"
          hint="On whatever the conversation says, keys or no keys."
          checked={entry.constant ?? false}
          pending={pending}
          onChange={(constant) => onChange({ constant })}
        />
        <Switch
          label="Needs a second key as well"
          hint="One of the keys below has to turn up too."
          checked={entry.selective ?? false}
          pending={pending}
          onChange={(selective) => onChange({ selective })}
        />
        <label className={styles.field}>
          <span>
            Second keys <small>one per line</small>
          </span>
          <textarea
            rows={3}
            value={writeKeys(entry.secondaryKeys)}
            onChange={(event) =>
              onChange({ secondaryKeys: readKeys(event.target.value) })
            }
            disabled={pending}
          />
        </label>
        <Switch
          label="Match the case of a key"
          hint="Off, ledger and Ledger both count."
          checked={entry.caseSensitive ?? false}
          pending={pending}
          onChange={(caseSensitive) => onChange({ caseSensitive })}
        />
      </fieldset>

      <fieldset className={styles.group}>
        <legend>Where it goes</legend>
        <div className={styles.pair}>
          <label className={styles.field}>
            <span>
              Order <small>among the entries that fired with it</small>
            </span>
            <input
              type="number"
              value={entry.order ?? 0}
              onChange={(event) =>
                onChange({ order: Number(event.target.value) || 0 })
              }
              disabled={pending}
            />
          </label>
          <label className={styles.field}>
            <span>Position</span>
            <select
              value={entry.position ?? ""}
              onChange={(event) =>
                onChange({
                  position:
                    event.target.value === ""
                      ? undefined
                      : (event.target.value as LorebookEntry["position"]),
                })
              }
              disabled={pending}
            >
              <option value="">Leave it to whatever reads the book</option>
              <option value="before_character">Before the character</option>
              <option value="after_character">After the character</option>
            </select>
          </label>
        </div>
      </fieldset>

      <fieldset className={styles.group}>
        <legend>Passes after the first</legend>
        <Switch
          label="Do not let this entry switch others on"
          checked={recursion.exclude ?? false}
          pending={pending}
          onChange={(exclude) =>
            onChange({ recursion: { ...recursion, exclude } })
          }
        />
        <Switch
          label="Do not let other entries switch this one on"
          checked={recursion.prevent ?? false}
          pending={pending}
          onChange={(prevent) =>
            onChange({ recursion: { ...recursion, prevent } })
          }
        />
        <Switch
          label="Hold it back until a later pass"
          checked={recursion.delayUntil ?? false}
          pending={pending}
          onChange={(delayUntil) =>
            onChange({ recursion: { ...recursion, delayUntil } })
          }
        />
      </fieldset>
    </div>
  );
}

function Switch({
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
