"use client";

import { useState } from "react";
import type { LorebookEntry } from "@/lib/api/query";
import {
  CollectionEditor,
  Field,
  FieldGroup,
  FieldPair,
  ItemFields,
  ItemHeading,
  NothingChosen,
  readLines,
  Switch,
  writeLines,
} from "./CollectionEditor";

export function entryName(entry: LorebookEntry, position: number): string {
  if (entry.name?.trim()) return entry.name;
  if (entry.keys.length > 0) return entry.keys.join(", ");
  return `Entry ${position + 1}`;
}

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
  const current = entries[selected];

  function replaceCurrent(changes: Partial<LorebookEntry>) {
    onChange(
      entries.map((entry, index) =>
        index === selected ? { ...entry, ...changes } : entry,
      ),
    );
  }

  return (
    <CollectionEditor
      noun="entry"
      emptyMessage="This book has no entries yet."
      pending={pending}
      selected={selected}
      onSelect={setSelected}
      onAdd={() =>
        onChange([...entries, { keys: [], text: "", enabled: true }])
      }
      rows={entries.map((entry, index) => ({
        name: entryName(entry, index),
        detail:
          entry.keys.length === 0
            ? "No keys"
            : entry.keys.slice(0, 4).join(" · "),
        off: !entry.enabled,
        search: [entryName(entry, index), entry.text, entry.keys.join(" ")]
          .join(" ")
          .toLowerCase(),
      }))}
    >
      {current ? (
        <EntryFields
          entry={current}
          position={selected}
          pending={pending}
          onChange={replaceCurrent}
          onRemove={() => {
            onChange(entries.filter((_, index) => index !== selected));
            setSelected(Math.max(0, Math.min(selected, entries.length - 2)));
          }}
        />
      ) : (
        <NothingChosen>
          Choose an entry to edit it, or add the first one.
        </NothingChosen>
      )}
    </CollectionEditor>
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
    <ItemFields>
      <ItemHeading
        name={entryName(entry, position)}
        noun="entry"
        pending={pending}
        onRemove={onRemove}
      />

      <Field label="Name" hint="optional, and never sent to a model">
        <input
          value={entry.name ?? ""}
          onChange={(event) =>
            onChange({ name: event.target.value || undefined })
          }
          disabled={pending}
        />
      </Field>

      <Field label="Entry text">
        <textarea
          rows={8}
          value={entry.text}
          onChange={(event) => onChange({ text: event.target.value })}
          disabled={pending}
        />
      </Field>

      <FieldGroup legend="What switches it on">
        <Field label="Keys" hint="one per line">
          <textarea
            rows={4}
            value={writeLines(entry.keys)}
            onChange={(event) =>
              onChange({ keys: readLines(event.target.value) })
            }
            disabled={pending}
          />
        </Field>
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
        <Field label="Second keys" hint="one per line">
          <textarea
            rows={3}
            value={writeLines(entry.secondaryKeys)}
            onChange={(event) =>
              onChange({ secondaryKeys: readLines(event.target.value) })
            }
            disabled={pending}
          />
        </Field>
        <Switch
          label="Match the case of a key"
          hint="Off, ledger and Ledger both count."
          checked={entry.caseSensitive ?? false}
          pending={pending}
          onChange={(caseSensitive) => onChange({ caseSensitive })}
        />
      </FieldGroup>

      <FieldGroup legend="Where it goes">
        <FieldPair>
          <Field label="Order" hint="among the entries that fired with it">
            <input
              type="number"
              value={entry.order ?? 0}
              onChange={(event) =>
                onChange({ order: Number(event.target.value) || 0 })
              }
              disabled={pending}
            />
          </Field>
          <Field label="Position">
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
          </Field>
        </FieldPair>
      </FieldGroup>

      <FieldGroup legend="Passes after the first">
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
      </FieldGroup>
    </ItemFields>
  );
}
