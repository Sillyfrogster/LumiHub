import { describe, expect, test } from "bun:test";
import {
  contentItemCount,
  fitsInTheSheet,
  INLINE_ITEM_LIMIT,
  LAYOUTS,
  layoutChoiceIssue,
  opensFullScreen,
  packBlockRows,
  WIDTH_COLUMNS,
  widthChoiceIssue,
} from "./page-arrangement";

type TestBlock = {
  id: string;
  width: keyof typeof WIDTH_COLUMNS;
  hidden: boolean;
};

function block(
  id: string,
  width: TestBlock["width"],
  hidden = false,
): TestBlock {
  return { id, width, hidden };
}

function spans(rows: ReturnType<typeof packBlockRows<TestBlock>>) {
  return rows.map((row) => row.map((item) => [item.block.id, item.columns]));
}

describe("page arrangement", () => {
  test("the four named widths are twelve, eight, six and four columns", () => {
    expect(WIDTH_COLUMNS).toEqual({
      full: 12,
      two_thirds: 8,
      half: 6,
      third: 4,
    });
  });

  test("the six layouts have finite named slots and minimum widths", () => {
    expect(LAYOUTS).toEqual({
      single: { slots: ["main"], minimumColumns: 4 },
      duo: { slots: ["left", "right"], minimumColumns: 8 },
      "main-aside": { slots: ["main", "aside"], minimumColumns: 8 },
      trio: { slots: ["left", "middle", "right"], minimumColumns: 12 },
      "stack-2": { slots: ["top", "bottom"], minimumColumns: 4 },
      "stack-3": { slots: ["top", "middle", "bottom"], minimumColumns: 4 },
    });
  });

  test("blocks stay in page order and start a new row when they do not fit", () => {
    expect(
      spans(
        packBlockRows(
          [block("a", "two_thirds"), block("b", "half"), block("c", "third")],
          { showHidden: true },
        ),
      ),
    ).toEqual([
      [["a", 12]],
      [
        ["b", 6],
        ["c", 6],
      ],
    ]);
  });

  test("the last block absorbs the remainder and a lone block fills its row", () => {
    expect(
      spans(
        packBlockRows(
          [block("a", "third"), block("b", "half"), block("c", "third")],
          { showHidden: true },
        ),
      ),
    ).toEqual([
      [
        ["a", 4],
        ["b", 8],
      ],
      [["c", 12]],
    ]);
  });

  test("a hidden block leaves reader packing and stays in owner packing", () => {
    const blocks = [block("hidden", "half", true), block("shown", "half")];

    expect(spans(packBlockRows(blocks, { showHidden: false }))).toEqual([
      [["shown", 12]],
    ]);
    expect(spans(packBlockRows(blocks, { showHidden: true }))).toEqual([
      [
        ["hidden", 6],
        ["shown", 6],
      ],
    ]);
  });

  test("every combination fills every completed row", () => {
    const widths = Object.keys(WIDTH_COLUMNS) as TestBlock["width"][];
    for (const first of widths) {
      for (const second of widths) {
        for (const third of widths) {
          const rows = packBlockRows(
            [block("a", first), block("b", second), block("c", third)],
            { showHidden: true },
          );
          for (const row of rows) {
            expect(row.reduce((total, item) => total + item.columns, 0)).toBe(
              12,
            );
          }
        }
      }
    }
  });

  test("narrow packing ignores declared widths", () => {
    expect(
      spans(
        packBlockRows([block("a", "third"), block("b", "half")], {
          showHidden: true,
          narrow: true,
        }),
      ),
    ).toEqual([[["a", 12]], [["b", 12]]]);
  });

  test("layout and width refusals name the first fix", () => {
    expect(
      layoutChoiceIssue("trio", "half", [
        "Description",
        "Personality",
        "Scenario",
      ]),
    ).toBe("Trio needs Full width. Widen this section first.");
    expect(widthChoiceIssue("trio", "half")).toBe(
      "Trio needs Full width. Choose another layout before narrowing this section.",
    );
    expect(
      layoutChoiceIssue("stack-2", "full", [
        "Greetings",
        "Example dialogue",
        "Group-only greetings",
      ]),
    ).toBe(
      "Stack 2 has no room for Group-only greetings. Move or remove it first.",
    );
  });
});

describe("remove confirmation counts", () => {
  test("counts each element through its public content shape", () => {
    expect(
      contentItemCount({ type: "prose", content: { text: "One body" } }),
    ).toBe(1);
    expect(
      contentItemCount({
        type: "text_set",
        content: { texts: [{ text: "First" }, { text: "Second" }] },
      }),
    ).toBe(2);
    expect(
      contentItemCount({
        type: "dialogue_sample",
        content: { turns: [{ speaker: "A", text: "Hello" }] },
      }),
    ).toBe(1);
    expect(
      contentItemCount({
        type: "image_set",
        content: { images: [{ mediaId: "one" }, { mediaId: "two" }] },
      }),
    ).toBe(2);
  });

  test("empty content counts as nothing lost", () => {
    expect(contentItemCount({ type: "prose", content: { text: "" } })).toBe(0);
    expect(contentItemCount({ type: "text_set", content: { texts: [] } })).toBe(
      0,
    );
  });

  test("counts every catalog element through its named collection", () => {
    const examples = [
      ["field_list", { fields: [{}, {}] }, 2],
      ["entry_table", { entries: [{}, {}, {}] }, 3],
      ["link_list", { links: [{}] }, 1],
      ["prompt_list", { fragments: [{}, {}] }, 2],
      ["variable_schema", { variables: { name: {}, mood: {} } }, 2],
      ["setting_group", { settings: { temperature: 1 } }, 1],
      ["script_list", { scripts: [{}, {}] }, 2],
      [
        "color_set",
        { colors: { light: { ink: "#000" }, dark: { ink: "#fff" } } },
        2,
      ],
      [
        "stylesheet_set",
        { global: "body {}", stylesheets: { card: ".card {}" } },
        2,
      ],
      ["record_list", { records: [{}, {}, {}] }, 3],
    ] as const;
    for (const [type, content, count] of examples) {
      expect(contentItemCount({ type, content })).toBe(count);
    }
  });
});

describe("where an element is edited", () => {
  test("the four collection types open a full-screen surface", () => {
    for (const type of [
      "entry_table",
      "image_set",
      "text_set",
      "dialogue_sample",
    ]) {
      expect(opensFullScreen(type)).toBe(true);
    }
    for (const type of ["prose", "field_list", "link_list"]) {
      expect(opensFullScreen(type)).toBe(false);
    }
  });

  test("small content stays editable in the sheet", () => {
    const two = { type: "text_set", content: { texts: [{}, {}] } };
    expect(fitsInTheSheet(two)).toBe(true);

    const many = {
      type: "entry_table",
      content: { entries: Array.from({ length: INLINE_ITEM_LIMIT + 1 }) },
    };
    expect(fitsInTheSheet(many)).toBe(false);
  });

  test("prose is never sent to the overlay however long it is", () => {
    expect(
      fitsInTheSheet({ type: "prose", content: { text: "x".repeat(9000) } }),
    ).toBe(true);
  });
});
