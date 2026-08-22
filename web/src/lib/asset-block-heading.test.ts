import { describe, expect, test } from "bun:test";
import { blockCounts, isTabularBlock } from "./asset-block-heading";

type TestElement = {
  type: string;
  isEmpty: boolean;
  facts: string[];
};

function element(
  type: string,
  facts: string[] = [],
  isEmpty = false,
): TestElement {
  return { type, facts, isEmpty };
}

describe("what a block says under its title", () => {
  test("a block with nothing to count says nothing", () => {
    expect(blockCounts([element("prose"), element("prose")])).toBe("");
  });

  test("the only element that counts speaks for itself", () => {
    expect(
      blockCounts([
        element("prose"),
        element("entry_table", ["1,004 entries", "1,003 switched on"]),
      ]),
    ).toBe("1,004 entries · 1,003 switched on");
  });

  test("several groups of the same thing are added up", () => {
    expect(
      blockCounts([
        element("setting_group", ["11 settings", "4 filled in"]),
        element("setting_group", ["3 settings"]),
        element("setting_group", ["12 settings", "2 filled in"]),
      ]),
    ).toBe("26 settings");
  });

  test("a total crossing a thousand keeps its comma", () => {
    expect(
      blockCounts([
        element("entry_table", ["999 entries"]),
        element("entry_table", ["5 entries"]),
      ]),
    ).toBe("1,004 entries");
  });

  test("different things keep their own nouns", () => {
    expect(
      blockCounts([
        element("text_set", ["4 greetings"]),
        element("text_set", ["1 group-only greeting"]),
      ]),
    ).toBe("4 greetings · 1 group-only greeting");
  });

  test("a singular total keeps the singular noun", () => {
    expect(
      blockCounts([
        element("image_set", ["1 image"]),
        element("text_set", ["2 items"]),
      ]),
    ).toBe("1 image · 2 items");
  });

  test("a total with no plural to write falls back to each count", () => {
    expect(
      blockCounts([
        element("entry_table", ["1 entry"]),
        element("entry_table", ["1 entry"]),
      ]),
    ).toBe("1 entry · 1 entry");
  });

  test("an empty element contributes nothing", () => {
    expect(
      blockCounts([
        element("entry_table", ["8 entries"], true),
        element("text_set", ["2 greetings"]),
      ]),
    ).toBe("2 greetings");
  });
});

describe("which blocks keep a surface", () => {
  test("a block holding a table is tabular", () => {
    expect(
      isTabularBlock([element("prose"), element("entry_table", ["8 entries"])]),
    ).toBe(true);
  });

  test("a block of prose is not", () => {
    expect(isTabularBlock([element("prose"), element("text_set")])).toBe(false);
  });

  test("an empty table does not earn a surface", () => {
    expect(isTabularBlock([element("entry_table", [], true)])).toBe(false);
  });
});
