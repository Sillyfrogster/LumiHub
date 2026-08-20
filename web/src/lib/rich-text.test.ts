import { describe, expect, test } from "bun:test";
import {
  type RichBlock,
  type RichInline,
  readRichText,
  richTextsOf,
} from "./rich-text";

/** The words a reader ends up with, one string per block. */
function lines(blocks: RichBlock[]): string[] {
  return blocks.flatMap((block) => {
    if (block.kind === "quote") return lines(block.children);
    if (block.kind === "list") return block.items.flatMap(lines);
    return [words(block.children)];
  });
}

function words(children: RichInline[]): string {
  return children
    .map((child) => {
      if (child.kind === "text") return child.text;
      if (child.kind === "code") return child.text;
      if (child.kind === "break") return "\n";
      return words(child.children);
    })
    .join("");
}

describe("restricted markdown", () => {
  test("keeps a plain paragraph exactly as it was written", () => {
    const rich = readRichText("She closes the book on a ribbon.");
    expect(rich.blocks).toEqual([
      {
        kind: "paragraph",
        children: [{ kind: "text", text: "She closes the book on a ribbon." }],
      },
    ]);
    expect(rich.formattingRemoved).toBe(false);
  });

  test("a blank line starts a paragraph and a single newline breaks a line", () => {
    const rich = readRichText("One\nstill one\n\nTwo");
    expect(lines(rich.blocks)).toEqual(["One\nstill one", "Two"]);
  });

  test("reads emphasis, strong, inline code and a link", () => {
    const rich = readRichText(
      "*soft* **loud** `exact` [home](https://illarin.xyz)",
    );
    expect(rich.blocks[0]).toEqual({
      kind: "paragraph",
      children: [
        { kind: "emphasis", children: [{ kind: "text", text: "soft" }] },
        { kind: "text", text: " " },
        { kind: "strong", children: [{ kind: "text", text: "loud" }] },
        { kind: "text", text: " " },
        { kind: "code", text: "exact" },
        { kind: "text", text: " " },
        {
          kind: "link",
          href: "https://illarin.xyz",
          children: [{ kind: "text", text: "home" }],
        },
      ],
    });
    expect(rich.formattingRemoved).toBe(false);
  });

  test("reads headings, both kinds of list, and a block quote", () => {
    const rich = readRichText(
      "## Her days\n\n- tea\n- toast\n\n3. third\n4. fourth\n\n> she said so",
    );
    expect(rich.blocks).toEqual([
      {
        kind: "heading",
        depth: 1,
        children: [{ kind: "text", text: "Her days" }],
      },
      {
        kind: "list",
        ordered: false,
        start: 1,
        items: [
          [{ kind: "paragraph", children: [{ kind: "text", text: "tea" }] }],
          [{ kind: "paragraph", children: [{ kind: "text", text: "toast" }] }],
        ],
      },
      {
        kind: "list",
        ordered: true,
        start: 3,
        items: [
          [{ kind: "paragraph", children: [{ kind: "text", text: "third" }] }],
          [{ kind: "paragraph", children: [{ kind: "text", text: "fourth" }] }],
        ],
      },
      {
        kind: "quote",
        children: [
          {
            kind: "paragraph",
            children: [{ kind: "text", text: "she said so" }],
          },
        ],
      },
    ]);
    expect(rich.formattingRemoved).toBe(false);
  });

  test("headings start one level down from the shallowest one written", () => {
    const rich = readRichText("### Her days\n\n#### Mornings\n\n###### Dawn");
    expect(rich.blocks.map((block) => "depth" in block && block.depth)).toEqual(
      [1, 2, 4],
    );
  });

  test("nothing beyond the supported set survives as formatting", () => {
    const image = readRichText("![a portrait of her](https://elsewhere/x.png)");
    expect(lines(image.blocks)).toEqual(["a portrait of her"]);
    expect(image.formattingRemoved).toBe(true);

    const fence = readRichText('```json\n{ "a": 1 }\n```');
    expect(lines(fence.blocks)).toEqual(['{ "a": 1 }']);
    expect(fence.formattingRemoved).toBe(true);

    const table = readRichText("| a | b |\n| - | - |\n| 1 | 2 |");
    expect(lines(table.blocks)).toEqual(["| a | b |\n| - | - |\n| 1 | 2 |"]);
    expect(table.formattingRemoved).toBe(false);

    const struck = readRichText("~~gone~~");
    expect(lines(struck.blocks)).toEqual(["~~gone~~"]);
    expect(struck.formattingRemoved).toBe(false);
  });

  test("a rule between scenes stays the characters the creator typed", () => {
    const rich = readRichText("scene one\n\n---\n\nscene two");
    expect(lines(rich.blocks)).toEqual(["scene one", "---", "scene two"]);
    expect(rich.formattingRemoved).toBe(false);
  });

  test("four spaces in front of a line is prose, not a code block", () => {
    const rich = readRichText("She waits.\n\n    And waits.");
    expect(lines(rich.blocks)).toEqual(["She waits.", "And waits."]);
    expect(rich.formattingRemoved).toBe(false);
  });
});

describe("HTML shown as words", () => {
  test("a greeting wrapped in a styled div reads as its words", () => {
    const rich = readRichText(
      '<div style="font-family: monospace; background: #1a1510;">She looks up.</div>',
    );
    expect(lines(rich.blocks)).toEqual(["She looks up."]);
    expect(rich.formattingRemoved).toBe(true);
  });

  test("indented markup keeps its words on their own lines", () => {
    const rich = readRichText(
      '<div class="card">\n    <p>She looks up.</p>\n    <p>Then down.</p>\n</div>',
    );
    expect(lines(rich.blocks)).toEqual(["She looks up.", "Then down."]);
  });

  test("a line break tag becomes a line break", () => {
    const rich = readRichText("One<br>Two<br/>Three");
    expect(lines(rich.blocks)).toEqual(["One\nTwo\nThree"]);
    expect(rich.formattingRemoved).toBe(true);
  });

  test("a script, a stylesheet and a comment leave nothing behind", () => {
    const rich = readRichText(
      "<script>alert('hi')</script><style>body { color: red }</style><!-- note -->Words.",
    );
    expect(lines(rich.blocks)).toEqual(["Words."]);
    expect(rich.formattingRemoved).toBe(true);
  });

  test("an event attribute leaves no trace of itself", () => {
    const rich = readRichText(
      '<img src="x" onerror="alert(1)">Still here.<a href="javascript:alert(1)">click</a>',
    );
    expect(JSON.stringify(rich.blocks)).not.toContain("onerror");
    expect(JSON.stringify(rich.blocks)).not.toContain("javascript");
    expect(lines(rich.blocks)).toEqual(["Still here.click"]);
    expect(rich.formattingRemoved).toBe(true);
  });

  test("a link only survives with a scheme a page can follow", () => {
    for (const url of [
      "javascript:alert(1)",
      "JavaScript:alert(1)",
      "java&#9;script:alert&#40;1&#41;",
      "data:text/html;base64,PHNjcmlwdD4=",
      "vbscript:msgbox",
      "//elsewhere.example",
    ]) {
      const rich = readRichText(`[click](${url})`);
      expect(rich.blocks).toEqual([
        { kind: "paragraph", children: [{ kind: "text", text: "click" }] },
      ]);
      expect(rich.formattingRemoved).toBe(true);
    }

    const bare = readRichText("<javascript:alert(1)>");
    expect(bare.blocks).toEqual([
      {
        kind: "paragraph",
        children: [{ kind: "text", text: "javascript:alert(1)" }],
      },
    ]);
    expect(bare.formattingRemoved).toBe(true);

    for (const url of [
      "https://illarin.xyz/a/1",
      "http://illarin.xyz",
      "mailto:her@example.com",
      "/browse",
    ]) {
      const rich = readRichText(`[click](${url})`);
      const [paragraph] = rich.blocks;
      if (paragraph.kind !== "paragraph") throw new Error("want a paragraph");
      expect(paragraph.children[0]).toEqual({
        kind: "link",
        href: url,
        children: [{ kind: "text", text: "click" }],
      });
    }
  });

  test("text that only looks like markup is left alone", () => {
    for (const source of [
      "a < b > c",
      "she typed <enter> and waited",
      "<3 forever",
      "<personality>warm</personality>",
      "5 < 6 and 7 > 6",
    ]) {
      const rich = readRichText(source);
      expect(lines(rich.blocks)).toEqual([source]);
      expect(rich.formattingRemoved).toBe(false);
    }
  });

  test("markdown inside markup still reads as markdown", () => {
    const rich = readRichText("<b>*soft*</b> and <span>**loud**</span>");
    expect(rich.blocks[0]).toEqual({
      kind: "paragraph",
      children: [
        { kind: "emphasis", children: [{ kind: "text", text: "soft" }] },
        { kind: "text", text: " and " },
        { kind: "strong", children: [{ kind: "text", text: "loud" }] },
      ],
    });
    expect(rich.formattingRemoved).toBe(true);
  });

  test("a character reference is the character it names", () => {
    const rich = readRichText("Tea &amp; toast &mdash; always");
    expect(lines(rich.blocks)).toEqual(["Tea & toast — always"]);
    expect(rich.formattingRemoved).toBe(false);
  });

  test("empty and blank sources read as nothing at all", () => {
    for (const source of ["", "   \n\n  ", "<div></div>"]) {
      expect(readRichText(source).blocks).toEqual([]);
    }
  });
});

describe("which of an element's text is prose", () => {
  test("a rich prose body is prose and a verbatim one is not", () => {
    const content = { text: "**loud**" };
    expect(richTextsOf({ type: "prose", display: "rich", content })).toEqual([
      "**loud**",
    ]);
    expect(
      richTextsOf({ type: "prose", display: "verbatim", content }),
    ).toEqual([]);
  });

  test("text sets, dialogue, entries, fields, notes and questions are prose", () => {
    expect(
      richTextsOf({
        type: "text_set",
        display: "rich",
        content: { texts: [{ text: "one" }, { text: "two" }] },
      }),
    ).toEqual(["one", "two"]);
    expect(
      richTextsOf({
        type: "dialogue_sample",
        content: { turns: [{ speaker: "Her", text: "one" }] },
      }),
    ).toEqual(["one"]);
    expect(
      richTextsOf({
        type: "entry_table",
        content: { entries: [{ text: "one" }, { text: "two" }] },
      }),
    ).toEqual(["one", "two"]);
    expect(
      richTextsOf({
        type: "field_list",
        content: { fields: [{ name: "Age", value: "24" }] },
      }),
    ).toEqual(["24"]);
    expect(
      richTextsOf({
        type: "link_list",
        content: {
          links: [{ url: "https://x", note: "a note" }, { url: "https://y" }],
        },
      }),
    ).toEqual(["a note"]);
    expect(
      richTextsOf({
        type: "variable_schema",
        content: {
          variables: [{ name: "tone", description: "How she sounds" }],
        },
      }),
    ).toEqual(["How she sounds"]);
  });

  test("a prompt fragment is the prompt, so it is not prose", () => {
    expect(
      richTextsOf({
        type: "prompt_list",
        content: { fragments: [{ text: "<instructions>Stay in character." }] },
      }),
    ).toEqual([]);
  });
});
