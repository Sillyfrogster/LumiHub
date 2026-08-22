import { describe, expect, test } from "bun:test";
import { emptyPageInvitation } from "./empty-page-invitation";

function invitation(coreBlocks: string[], canAdd = true) {
  return emptyPageInvitation({ coreBlocks, canAdd, kindLabel: "character" });
}

describe("the invitation on an owner's empty page", () => {
  test("one block is named on its own", () => {
    expect(invitation(["Entries"])).toStartWith(
      "Fill in Entries to give the page something to show.",
    );
  });

  test("two blocks are joined with and", () => {
    expect(invitation(["The character", "Messages"])).toStartWith(
      "Fill in The character and Messages to give the page something to show.",
    );
  });

  test("three blocks read as a list", () => {
    expect(invitation(["One", "Two", "Three"])).toStartWith(
      "Fill in One, Two and Three to give the page something to show.",
    );
  });

  test("it names Add block only when there is something to add", () => {
    expect(invitation(["Entries"], true)).toContain("Add block");
    expect(invitation(["Entries"], false)).not.toContain("Add block");
  });

  test("with no blocks yet it points at the only control there is", () => {
    expect(invitation([], true)).toBe(
      "Add block brings in the first of what a character can hold.",
    );
  });

  test("a kind Illarin cannot build yet says so, and says the file is kept", () => {
    expect(invitation([], false)).toBe(
      "Illarin has no blocks for a character yet. The file you uploaded is kept whole, and every download carries it.",
    );
  });
});
