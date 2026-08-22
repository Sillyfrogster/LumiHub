"use client";

import { Search } from "lucide-react";
import {
  type KeyboardEvent,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { ChipSet } from "@/components/ui/Chip";
import { RichText } from "@/components/ui/RichText";
import type { LorebookEntry } from "@/lib/api/query";
import {
  type EntryPresentation,
  type EntrySort,
  type LorebookIndex,
  readLorebook,
} from "@/lib/lorebook-entry";
import styles from "./Lorebook.module.css";

/** Enough keys to recognise an entry by. The rest are one press away. */
const KEY_PREVIEW_LIMIT = 6;

/**
 * A lorebook, as the book it is: an index of every entry it holds, and the one
 * a reader picked open beside it. The index scrolls inside itself, so a book of
 * a thousand entries is the same height on the page as a book of five.
 */
export function Lorebook({ entries }: { entries: LorebookEntry[] }) {
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<EntrySort>("book");
  const [includeOff, setIncludeOff] = useState(true);
  const [chosen, setChosen] = useState<string | null>(null);
  const names = useId();

  const book = useMemo(
    () => readLorebook(entries, { search, sort, includeOff }),
    [entries, search, sort, includeOff],
  );

  const shown =
    book.entries.find((entry) => entry.id === chosen) ?? book.entries[0];

  return (
    <div className={styles.lorebook}>
      <Controls
        search={search}
        sort={sort}
        includeOff={includeOff}
        book={book}
        onSearch={setSearch}
        onSort={setSort}
        onIncludeOff={setIncludeOff}
      />
      {book.entries.length === 0 ? (
        <Nothing search={search} book={book} />
      ) : (
        <div className={styles.body}>
          <Index
            entries={book.entries}
            names={names}
            shownId={shown?.id}
            onChoose={setChosen}
          />
          {shown ? (
            <Entry entry={shown} labelledBy={`${names}-${shown.id}`} />
          ) : null}
        </div>
      )}
    </div>
  );
}

/**
 * Every entry the view holds, as one tab stop with arrow keys through it. A
 * reader tabbing past a book of 285 entries would otherwise press tab 285
 * times.
 */
function Index({
  entries,
  names,
  shownId,
  onChoose,
}: {
  entries: EntryPresentation[];
  names: string;
  shownId: string | undefined;
  onChoose: (id: string) => void;
}) {
  const index = useRef<HTMLDivElement>(null);

  /* Sorting or searching can leave the open entry outside the rows on show. */
  useEffect(() => {
    const list = index.current;
    const row = shownId
      ? list?.querySelector(`[data-row="${CSS.escape(shownId)}"]`)
      : null;
    if (!list || !row) return;
    const rows = list.getBoundingClientRect();
    const chosenRow = row.getBoundingClientRect();
    if (chosenRow.top < rows.top) {
      list.scrollTop -= rows.top - chosenRow.top;
    } else if (chosenRow.bottom > rows.bottom) {
      list.scrollTop += chosenRow.bottom - rows.bottom;
    }
  }, [shownId]);

  function moveFocus(event: KeyboardEvent<HTMLButtonElement>, from: number) {
    const to = nextRow(event.key, from, entries.length);
    if (to === null) return;
    event.preventDefault();
    index.current?.querySelectorAll("button")[to]?.focus();
  }

  return (
    <div
      className={styles.index}
      ref={index}
      role="tablist"
      aria-label="The entries in this book"
      aria-orientation="vertical"
    >
      {entries.map((entry, row) => (
        <button
          key={entry.id}
          type="button"
          role="tab"
          id={`${names}-${entry.id}`}
          aria-selected={entry.id === shownId}
          tabIndex={entry.id === shownId ? 0 : -1}
          data-row={entry.id}
          data-off={entry.isOff ? true : undefined}
          onFocus={() => onChoose(entry.id)}
          onClick={() => onChoose(entry.id)}
          onKeyDown={(event) => moveFocus(event, row)}
        >
          <span className={styles.rowName} title={entry.name}>
            {entry.name}
          </span>
          <span className={styles.rowNote}>{entry.note}</span>
        </button>
      ))}
    </div>
  );
}

/** Why the index shows nothing, which is not always the same reason. */
function Nothing({ search, book }: { search: string; book: LorebookIndex }) {
  const wanted = search.trim();
  if (wanted !== "") {
    return (
      <p className={styles.nothing}>
        Nothing here is named <strong>{wanted}</strong>, and no key holds it.
      </p>
    );
  }
  return (
    <p className={styles.nothing}>
      {book.total === 0
        ? "This book holds no entries yet."
        : "Every entry in this book is switched off."}
    </p>
  );
}

function Controls({
  search,
  sort,
  includeOff,
  book,
  onSearch,
  onSort,
  onIncludeOff,
}: {
  search: string;
  sort: EntrySort;
  includeOff: boolean;
  book: LorebookIndex;
  onSearch: (search: string) => void;
  onSort: (sort: EntrySort) => void;
  onIncludeOff: (includeOff: boolean) => void;
}) {
  const searchField = useId();
  const sortField = useId();

  return (
    <div className={styles.controls}>
      <div className={styles.search}>
        <Search size={16} aria-hidden="true" />
        <label className={styles.srOnly} htmlFor={searchField}>
          Search the entries by name and by key
        </label>
        <input
          id={searchField}
          type="search"
          value={search}
          placeholder="Search names and keys"
          onChange={(event) => onSearch(event.target.value)}
        />
      </div>
      <div className={styles.sort}>
        <label htmlFor={sortField}>Order</label>
        <select
          id={sortField}
          value={sort}
          onChange={(event) => onSort(event.target.value as EntrySort)}
        >
          <option value="book">As the book holds them</option>
          <option value="name">By name, A to Z</option>
        </select>
      </div>
      {book.off > 0 ? (
        <label className={styles.includeOff}>
          <input
            type="checkbox"
            checked={includeOff}
            onChange={(event) => onIncludeOff(event.target.checked)}
          />
          Include the {book.off} that {book.off === 1 ? "is" : "are"} off
        </label>
      ) : null}
      <p className={styles.showing}>
        {book.entries.length === book.total
          ? null
          : `${book.entries.length} of ${book.total}`}
      </p>
    </div>
  );
}

// biome-ignore-start lint/a11y/noNoninteractiveTabindex: A tab panel holding
// nothing focusable has to be reachable itself, or the entry a reader just
// picked is the one thing the keyboard cannot get to.
function Entry({
  entry,
  labelledBy,
}: {
  entry: EntryPresentation;
  labelledBy: string;
}) {
  return (
    <div
      className={styles.entry}
      role="tabpanel"
      aria-labelledby={labelledBy}
      tabIndex={0}
    >
      {/* A name taken from the opening would say the text's first line twice. */}
      {entry.named === "opening" ? null : (
        <h4 className={styles.entryName}>{entry.name}</h4>
      )}
      <p className={styles.firing}>{entry.firing.join(" · ")}</p>
      {entry.keys.length > 0 ? (
        <ChipSet
          className={styles.keys}
          limit={KEY_PREVIEW_LIMIT}
          items={entry.keys}
        />
      ) : null}
      {entry.secondaryKeys.length > 0 ? (
        <div className={styles.secondary}>
          <p>Second keys</p>
          <ChipSet limit={KEY_PREVIEW_LIMIT} items={entry.secondaryKeys} />
        </div>
      ) : null}
      <RichText text={entry.text} className={styles.text} />
    </div>
  );
}
// biome-ignore-end lint/a11y/noNoninteractiveTabindex: The panel ends here.

/** Where an arrow key sends focus in the index, or nothing if it is not one. */
function nextRow(key: string, from: number, rows: number): number | null {
  if (rows === 0) return null;
  if (key === "ArrowDown") return Math.min(from + 1, rows - 1);
  if (key === "ArrowUp") return Math.max(from - 1, 0);
  if (key === "Home") return 0;
  if (key === "End") return rows - 1;
  return null;
}
