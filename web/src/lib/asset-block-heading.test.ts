import { describe, expect, test } from "bun:test";
import { blockCounts } from "./asset-block-heading";

type TestElement = {
  isEmpty: boolean;
  facts: string[];
};

function element(facts: string[] = [], isEmpty = false): TestElement {
  return { facts, isEmpty };
}

describe("what a block says under its title", () => {
  test("a block with nothing to count says nothing", () => {
    expect(blockCounts([element(), element()])).toBe("");
  });

  test("the only element that counts speaks for itself", () => {
    expect(
      blockCounts([element(), element(["1,004 entries", "1,003 switched on"])]),
    ).toBe("1,004 entries · 1,003 switched on");
  });

  test("several groups of the same thing are added up", () => {
    expect(
      blockCounts([
        element(["11 settings", "4 filled in"]),
        element(["3 settings"]),
        element(["12 settings", "2 filled in"]),
      ]),
    ).toBe("26 settings");
  });

  test("a total crossing a thousand keeps its comma", () => {
    expect(
      blockCounts([element(["999 entries"]), element(["5 entries"])]),
    ).toBe("1,004 entries");
  });

  test("different things keep their own nouns", () => {
    expect(
      blockCounts([
        element(["4 greetings"]),
        element(["1 group-only greeting"]),
      ]),
    ).toBe("4 greetings · 1 group-only greeting");
  });

  test("a singular total keeps the singular noun", () => {
    expect(blockCounts([element(["1 image"]), element(["2 items"])])).toBe(
      "1 image · 2 items",
    );
  });

  test("a total with no plural to write falls back to each count", () => {
    expect(blockCounts([element(["1 entry"]), element(["1 entry"])])).toBe(
      "1 entry · 1 entry",
    );
  });

  test("an empty element contributes nothing", () => {
    expect(
      blockCounts([element(["8 entries"], true), element(["2 greetings"])]),
    ).toBe("2 greetings");
  });
});
