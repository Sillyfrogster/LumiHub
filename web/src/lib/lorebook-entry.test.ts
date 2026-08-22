import { describe, expect, test } from "bun:test";
import { type LorebookView, readEntry, readLorebook } from "./lorebook-entry";

const on = { keys: ["ramiel"], enabled: true, text: "" };

const showing: LorebookView = { search: "", sort: "book", includeOff: true };

describe("how an entry names itself", () => {
  test("an entry with a name uses it", () => {
    expect(readEntry({ ...on, name: "Remielle Dan" }, 1).name).toBe(
      "Remielle Dan",
    );
  });

  test("a name somebody wrote is recorded as one", () => {
    expect(readEntry({ ...on, name: "Remielle Dan" }, 1).named).toBe("written");
  });

  test("an entry nobody named is called after the key that fires it", () => {
    expect(readEntry({ ...on, keys: ["Ramiel", "void hunter"] }, 7).name).toBe(
      "Ramiel",
    );
    expect(readEntry({ ...on, name: "   ", keys: ["Ramiel"] }, 7).name).toBe(
      "Ramiel",
    );
  });

  test("an entry with no name and no key is called after how it opens", () => {
    expect(
      readEntry(
        { keys: [], enabled: true, text: "**Shinsekai** holds 7.2 million." },
        7,
      ).name,
    ).toBe("Shinsekai holds 7.2 million.");
  });

  test("a name taken from the text is recorded as one, so nothing says it twice", () => {
    expect(
      readEntry({ keys: [], enabled: true, text: "Shinsekai holds 7.2m." }, 7)
        .named,
    ).toBe("opening");
    expect(readEntry({ ...on, keys: ["ramiel"] }, 7).named).toBe("key");
    expect(readEntry({ keys: [], enabled: true, text: " " }, 7).named).toBe(
      "position",
    );
  });

  test("an opening too long to be a name is cut at a word", () => {
    expect(
      readEntry(
        {
          keys: [],
          enabled: true,
          text: "Population Composition: Shinsekai maintains a permanent population of 7.2 million residents.",
        },
        7,
      ).name,
    ).toBe("Population Composition: Shinsekai maintains a\u2026");
  });

  test("an entry with nothing at all falls back to its place in the book", () => {
    expect(readEntry({ keys: [], enabled: true, text: "  " }, 7).name).toBe(
      "Entry 7",
    );
  });
});

describe("what the index says beside a name", () => {
  test("an ordinary entry says how many keys fire it", () => {
    expect(readEntry(on, 1).note).toBe("1 key");
    expect(readEntry({ ...on, keys: ["a", "b"] }, 1).note).toBe("2 keys");
  });

  test("an entry that is off says so", () => {
    expect(readEntry({ ...on, enabled: false }, 1).note).toBe("Off");
  });

  test("an entry that fires whatever the conversation says so", () => {
    expect(readEntry({ ...on, constant: true }, 1).note).toBe("Always on");
  });

  test("being off outranks being constant, because it fires neither way", () => {
    expect(readEntry({ ...on, constant: true, enabled: false }, 1).note).toBe(
      "Off",
    );
  });

  test("an entry with no keys says that, which is not the same as none shown", () => {
    expect(readEntry({ ...on, keys: [] }, 1).note).toBe("No keys");
  });
});

describe("the keys an entry shows", () => {
  test("a key repeated in the same entry is still its own chip", () => {
    expect(readEntry({ ...on, keys: ["fire", "fire"] }, 1).keys).toEqual([
      { id: "0-fire", label: "fire" },
      { id: "1-fire", label: "fire" },
    ]);
  });

  test("a key of nothing but space is not a chip", () => {
    expect(readEntry({ ...on, keys: ["fire", " "] }, 1).keys).toEqual([
      { id: "0-fire", label: "fire" },
    ]);
  });

  test("secondary keys count only where the entry uses them", () => {
    const selective = { ...on, selective: true, secondaryKeys: ["night"] };
    expect(readEntry(selective, 1).secondaryKeys).toEqual([
      { id: "0-night", label: "night" },
    ]);
    expect(
      readEntry({ ...selective, selective: false }, 1).secondaryKeys,
    ).toEqual([]);
  });
});

describe("what decides whether an entry fires", () => {
  test("one key reads as one key", () => {
    expect(readEntry(on, 1).firing).toEqual(["Switched on by its key"]);
  });

  test("several keys are counted", () => {
    expect(readEntry({ ...on, keys: ["a", "b", "c"] }, 1).firing).toEqual([
      "Switched on by any of its 3 keys",
    ]);
  });

  test("an entry with no keys and no constant flag fires on nothing", () => {
    expect(readEntry({ ...on, keys: [] }, 1).firing).toEqual([
      "No keys, so nothing switches it on",
    ]);
  });

  test("an entry that is off says so before anything else", () => {
    expect(readEntry({ ...on, enabled: false }, 1).firing[0]).toBe(
      "Switched off",
    );
  });

  test("the rules a book bothers to set are all said", () => {
    expect(
      readEntry(
        {
          ...on,
          constant: true,
          caseSensitive: true,
          position: "after_character",
          recursion: { exclude: true, prevent: true, delayUntil: true },
        },
        1,
      ).firing,
    ).toEqual([
      "Always on",
      "Matches the case of its keys",
      "Goes in after the character",
      "Other entries cannot switch it on",
      "Cannot switch other entries on",
      "Held back until a later pass",
    ]);
  });

  test("an order every entry shares is said about none of them", () => {
    const shared = [
      { ...on, order: 100 },
      { ...on, order: 100 },
    ];
    expect(readLorebook(shared, showing).entries[0].firing).toEqual([
      "Switched on by its key",
    ]);
  });

  test("an order that sets one entry apart is said", () => {
    const mixed = [
      { ...on, order: 100 },
      { ...on, order: 4 },
    ];
    expect(readLorebook(mixed, showing).entries[1].firing).toEqual([
      "Switched on by its key",
      "Order 4",
    ]);
  });
});

describe("the index a reader searches", () => {
  const book = [
    { ...on, id: "a", name: "Remielle Dan", keys: ["ramiel", "void hunter"] },
    { ...on, id: "b", name: "Belle", keys: ["belle"], enabled: false },
    { ...on, id: "c", name: "Aokigahara", keys: ["forest"] },
  ];

  test("the whole book is counted whatever the view shows", () => {
    const read = readLorebook(book, { ...showing, search: "belle" });
    expect(read.total).toBe(3);
    expect(read.off).toBe(1);
    expect(read.entries).toHaveLength(1);
  });

  test("a search matches a name", () => {
    expect(
      readLorebook(book, { ...showing, search: "remielle" }).entries.map(
        (entry) => entry.id,
      ),
    ).toEqual(["a"]);
  });

  test("a search matches a key, which is how a reader finds an entry it never names", () => {
    expect(
      readLorebook(book, { ...showing, search: "FOREST" }).entries.map(
        (entry) => entry.id,
      ),
    ).toEqual(["c"]);
  });

  test("a search matching nothing shows nothing", () => {
    expect(
      readLorebook(book, { ...showing, search: "kraken" }).entries,
    ).toEqual([]);
  });

  test("entries that are off can be folded away", () => {
    expect(
      readLorebook(book, { ...showing, includeOff: false }).entries.map(
        (entry) => entry.id,
      ),
    ).toEqual(["a", "c"]);
  });

  test("book order is the order the entries were written in", () => {
    expect(
      readLorebook(book, showing).entries.map((entry) => entry.id),
    ).toEqual(["a", "b", "c"]);
  });

  test("sorting by name leaves the positions saying where each entry really sits", () => {
    const sorted = readLorebook(book, { ...showing, sort: "name" });
    expect(sorted.entries.map((entry) => entry.name)).toEqual([
      "Aokigahara",
      "Belle",
      "Remielle Dan",
    ]);
    expect(sorted.entries.map((entry) => entry.position)).toEqual([3, 2, 1]);
  });

  test("an entry with no id of its own still gets one the index can address", () => {
    expect(readLorebook([on, on], showing).entries.map((e) => e.id)).toEqual([
      "entry-1",
      "entry-2",
    ]);
  });
});
