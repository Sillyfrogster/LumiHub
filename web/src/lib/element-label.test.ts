import { describe, expect, test } from "bun:test";
import { elementLabel } from "./element-label";

const pair = { elements: 2 };

describe("the label an element carries", () => {
  test("an element that tells itself from a sibling keeps its label", () => {
    expect(
      elementLabel(
        { role: "greetings", label: "Greetings" },
        { title: "Messages", ...pair },
      ),
    ).toBe("Greetings");
  });

  test("a block holding one element has already named it", () => {
    expect(
      elementLabel(
        { role: "gallery", label: "Images" },
        { title: "Gallery", elements: 1 },
      ),
    ).toBeNull();
  });

  test("an element with no role carries the name of its type, which says nothing", () => {
    expect(
      elementLabel({ label: "Details" }, { title: "Attributes", ...pair }),
    ).toBeNull();
    expect(
      elementLabel({ role: null, label: "Text" }, { title: "Usage", ...pair }),
    ).toBeNull();
  });

  test("a label with no words is no label", () => {
    expect(
      elementLabel(
        { role: "greetings", label: "  " },
        { title: "Messages", ...pair },
      ),
    ).toBeNull();
    expect(
      elementLabel({ role: "greetings" }, { title: "Messages", ...pair }),
    ).toBeNull();
  });
});

describe("a label the block title already says", () => {
  const under = (label: string, title: string | undefined) =>
    elementLabel({ role: "some_role", label }, { title, ...pair });

  test("the same word is a repeat", () => {
    expect(under("Entries", "Entries")).toBeNull();
  });

  test("case is not a difference", () => {
    expect(under("nudges", "Nudges")).toBeNull();
  });

  test("either apostrophe reads as the same word", () => {
    expect(under("Author’s notes", "Author's notes")).toBeNull();
  });

  test("the singular and the plural are the same word", () => {
    expect(under("Entries", "Entry")).toBeNull();
    expect(under("Greeting", "Greetings")).toBeNull();
  });

  test("a title that already contains the label is a repeat", () => {
    expect(under("Images", "Gallery images")).toBeNull();
  });

  test("a label that says more than the title is kept", () => {
    expect(under("Author’s notes", "Notes")).toBe("Author’s notes");
  });

  test("a different word is kept", () => {
    expect(under("Greetings", "Messages")).toBe("Greetings");
    expect(under("Samplers", "Settings")).toBe("Samplers");
    expect(under("Images", "Gallery")).toBe("Images");
  });

  test("a word inside another word is not the same word", () => {
    expect(under("Note", "Notebook")).toBe("Note");
  });

  test("a block with no title of its own repeats nothing", () => {
    expect(under("Description", undefined)).toBe("Description");
    expect(under("Description", "")).toBe("Description");
  });
});
