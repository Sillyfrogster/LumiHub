import { describe, expect, test } from "bun:test";
import {
  contentItemCount,
  fitsInTheSheet,
  INLINE_ITEM_LIMIT,
  LAYOUTS,
  layoutChoiceIssue,
  NARROW_BLOCK_GRID_PX,
  opensFullScreen,
  PROSE_MEASURE,
  packBlockRows,
  proseMeasureForWidth,
  WIDTH_COLUMNS,
  WIDTH_FLOORS_PX,
  widthChoiceIssue,
} from "./page-arrangement";

type TestBlock = {
  id: string;
  width: keyof typeof WIDTH_COLUMNS;
  hidden: boolean;
  empty: boolean;
};

function block(
  id: string,
  width: TestBlock["width"],
  hidden = false,
  empty = false,
): TestBlock {
  return { id, width, hidden, empty };
}

function spans(rows: ReturnType<typeof packBlockRows<TestBlock>>) {
  return rows.map((row) => row.map((item) => [item.block.id, item.columns]));
}

function placements(rows: ReturnType<typeof packBlockRows<TestBlock>>) {
  return rows.map((row) =>
    row.map((item) => [item.block.id, item.columns, item.startColumn]),
  );
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
          {},
        ),
      ),
    ).toEqual([
      [["a", 8]],
      [
        ["b", 6],
        ["c", 4],
      ],
    ]);
  });

  test("a block keeps its declared width and a short row stays short", () => {
    expect(
      spans(
        packBlockRows(
          [block("a", "third"), block("b", "half"), block("c", "third")],
          {},
        ),
      ),
    ).toEqual([
      [
        ["a", 4],
        ["b", 6],
      ],
      [["c", 4]],
    ]);
  });

  test("hidden and empty blocks keep their grid places when not rendered", () => {
    const blocks = [block("hidden", "half", true), block("shown", "half")];

    expect(placements(packBlockRows(blocks, {}))).toEqual([
      [
        ["hidden", 6, 1],
        ["shown", 6, 7],
      ],
    ]);

    const withEmpty = [
      block("first", "third"),
      block("empty", "half", false, true),
      block("last", "third"),
    ];
    expect(placements(packBlockRows(withEmpty, {}))).toEqual([
      [
        ["first", 4, 1],
        ["empty", 6, 5],
      ],
      [["last", 4, 1]],
    ]);
  });

  test("every width combination keeps each block at its declared size", () => {
    const widths = Object.keys(WIDTH_COLUMNS) as TestBlock["width"][];
    for (const first of widths) {
      for (const second of widths) {
        for (const third of widths) {
          const chosen = [first, second, third];
          const rows = packBlockRows(
            [block("a", first), block("b", second), block("c", third)],
            {},
          );
          const packed = rows.flat();
          expect(packed).toHaveLength(3);
          packed.forEach((item, index) => {
            expect(item.columns).toBe(WIDTH_COLUMNS[chosen[index]]);
          });
          for (const row of rows) {
            const occupied = row.reduce(
              (total, item) => total + item.columns,
              0,
            );
            expect(occupied).toBeLessThanOrEqual(12);
          }
        }
      }
    }
  });

  test("each width promotes at its pixel floor through the same ladder", () => {
    expect(WIDTH_FLOORS_PX).toEqual({
      full: 280,
      two_thirds: 640,
      half: 440,
      third: 320,
    });

    const columnsAt = (width: TestBlock["width"], availableWidth: number) =>
      packBlockRows([block("one", width)], {
        availableWidth,
      })[0][0].columns;

    expect(columnsAt("third", 1000)).toBe(4);
    expect(columnsAt("third", 999)).toBe(6);
    expect(columnsAt("third", 899)).toBe(12);
    expect(columnsAt("half", 900)).toBe(6);
    expect(columnsAt("half", 899)).toBe(12);
    expect(columnsAt("two_thirds", 970)).toBe(8);
    expect(columnsAt("two_thirds", 969)).toBe(12);
    expect(columnsAt("full", 280)).toBe(12);
  });

  test("block width never changes the measure of prose inside it", () => {
    const widths = ["full", "two_thirds", "half", "third"] as const;

    expect(PROSE_MEASURE).toBe("70ch");
    expect(widths.map(proseMeasureForWidth)).toEqual([
      "70ch",
      "70ch",
      "70ch",
      "70ch",
    ]);
  });

  test("narrow packing ignores declared widths", () => {
    expect(
      spans(
        packBlockRows([block("a", "third"), block("b", "half")], {
          availableWidth: NARROW_BLOCK_GRID_PX,
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
  test("every collection type opens a full-screen surface", () => {
    for (const type of [
      "entry_table",
      "image_set",
      "text_set",
      "dialogue_sample",
      "prompt_list",
      "variable_schema",
      "setting_group",
      "script_list",
    ]) {
      expect(opensFullScreen(type)).toBe(true);
    }
    for (const type of ["prose", "field_list", "link_list"]) {
      expect(opensFullScreen(type)).toBe(false);
    }
  });

  test("a seeded settings group is past what a sheet holds", () => {
    const seeded = {
      type: "setting_group",
      content: {
        settings: Array.from({ length: 18 }, (_, index) => ({
          name: `setting_${index}`,
          type: "number",
        })),
      },
    };
    expect(fitsInTheSheet(seeded)).toBe(false);

    const advanced = {
      type: "setting_group",
      content: { settings: [{ name: "seed", type: "number" }] },
    };
    expect(fitsInTheSheet(advanced)).toBe(true);
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
