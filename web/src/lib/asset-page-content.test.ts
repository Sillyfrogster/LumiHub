import { describe, expect, test } from "bun:test";
import {
  assetHoldsNothing,
  coreBlockTitles,
  rendersOnThePage,
  splitAssetPageContent,
} from "./asset-page-content";

type TestElement = {
  id: string;
  role?: string;
  isEmpty: boolean;
};

type TestBlock = {
  id: string;
  title: string;
  hidden: boolean;
  elements: TestElement[];
};

function element(id: string, role?: string, isEmpty = false): TestElement {
  return { id, role, isEmpty };
}

function block(id: string, elements: TestElement[], hidden = false): TestBlock {
  return { id, title: id, hidden, elements };
}

describe("reader page content", () => {
  test("Character, Voice and Scenario stay in the character block", () => {
    const character = block("The character", [
      element("character", "description"),
      element("voice", "personality"),
      element("scenario", "scenario"),
    ]);

    const result = splitAssetPageContent([character]);

    expect(result.publicBlocks[0].elements.map(({ id }) => id)).toEqual([
      "character",
      "voice",
      "scenario",
    ]);
    expect(result.publicBlocks[0].empty).toBe(false);
    expect(result.modelContent).toEqual([]);
  });

  test("only system and post-history instructions move to the disclosure", () => {
    const instructions = block("Model instructions", [
      element("system", "system_prompt"),
      element("post-history", "post_history_instructions"),
    ]);

    const result = splitAssetPageContent([instructions]);

    expect(result.publicBlocks[0].elements).toEqual([]);
    expect(result.publicBlocks[0].empty).toBe(true);
    expect(result.modelContent.map(({ element: item }) => item.id)).toEqual([
      "system",
      "post-history",
    ]);
  });

  test("ordinary public content remains in its block and empty content does not render", () => {
    const changelog = block("Changelog", [
      element("written"),
      element("empty", undefined, true),
    ]);

    const result = splitAssetPageContent([changelog]);

    expect(result.publicBlocks[0].elements.map(({ id }) => id)).toEqual([
      "written",
    ]);
    expect(result.publicBlocks[0].empty).toBe(false);
  });

  test("ordinary and disclosed elements split without emptying their block", () => {
    const mixed = block("Mixed", [
      element("notes"),
      element("system", "system_prompt"),
    ]);

    const result = splitAssetPageContent([mixed]);

    expect(result.publicBlocks[0].elements.map(({ id }) => id)).toEqual([
      "notes",
    ]);
    expect(result.publicBlocks[0].empty).toBe(false);
    expect(result.modelContent.map(({ element: item }) => item.id)).toEqual([
      "system",
    ]);
  });

  test("a hidden block contributes nothing to the model disclosure", () => {
    const hidden = block(
      "Hidden instructions",
      [element("system", "system_prompt")],
      true,
    );

    expect(splitAssetPageContent([hidden]).modelContent).toEqual([]);
  });
});

describe("the blocks a reader's page arranges", () => {
  test("a block with nothing to show holds no place in a row", () => {
    const { publicBlocks } = splitAssetPageContent([
      block("Prompt fragments", [element("fragments", "prompt_fragments")]),
      block("Nudges", [element("nudges", "prompt_nudges", true)]),
      block("Variables", [element("variables", "prompt_variables")]),
    ]);

    expect(publicBlocks.filter(rendersOnThePage).map(({ id }) => id)).toEqual([
      "Prompt fragments",
      "Variables",
    ]);
  });

  test("a hidden block holds no place either", () => {
    const { publicBlocks } = splitAssetPageContent([
      block("Usage", [element("usage")], true),
    ]);

    expect(publicBlocks.filter(rendersOnThePage)).toEqual([]);
  });
});

describe("an asset that holds nothing", () => {
  type Fillable = { title: string; required: boolean; isEmpty: boolean };
  const core = (title: string): Fillable => ({
    title,
    required: true,
    isEmpty: true,
  });
  const extra = (title: string): Fillable => ({
    title,
    required: false,
    isEmpty: true,
  });

  test("every block empty means the asset holds nothing", () => {
    expect(assetHoldsNothing([core("The character"), core("Messages")])).toBe(
      true,
    );
  });

  test("one written block is enough to stop that", () => {
    expect(
      assetHoldsNothing([
        { title: "The character", required: true, isEmpty: false },
        core("Messages"),
      ]),
    ).toBe(false);
  });

  test("an asset with no blocks at all holds nothing", () => {
    expect(assetHoldsNothing([])).toBe(true);
  });

  test("the invitation names the required blocks in page order", () => {
    expect(
      coreBlockTitles([
        core("The character"),
        extra("Changelog"),
        core("Messages"),
      ]),
    ).toEqual(["The character", "Messages"]);
  });

  test("with nothing required it names the blocks that are there", () => {
    expect(coreBlockTitles([extra("Entries"), extra("Changelog")])).toEqual([
      "Entries",
      "Changelog",
    ]);
  });

  test("it names at most three so the invitation stays one sentence", () => {
    expect(
      coreBlockTitles([core("One"), core("Two"), core("Three"), core("Four")]),
    ).toEqual(["One", "Two", "Three"]);
  });
});
