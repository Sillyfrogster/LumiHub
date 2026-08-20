export const WIDTH_COLUMNS = {
  full: 12,
  two_thirds: 8,
  half: 6,
  third: 4,
} as const;

export type BlockWidth = keyof typeof WIDTH_COLUMNS;

export const WIDTH_LABELS: Record<BlockWidth, string> = {
  full: "Full",
  two_thirds: "Two thirds",
  half: "Half",
  third: "A third",
};

export const LAYOUTS = {
  single: { slots: ["main"], minimumColumns: 4 },
  duo: { slots: ["left", "right"], minimumColumns: 8 },
  "main-aside": { slots: ["main", "aside"], minimumColumns: 8 },
  trio: { slots: ["left", "middle", "right"], minimumColumns: 12 },
  "stack-2": { slots: ["top", "bottom"], minimumColumns: 4 },
  "stack-3": { slots: ["top", "middle", "bottom"], minimumColumns: 4 },
} as const;

export type BlockLayout = keyof typeof LAYOUTS;

export const LAYOUT_LABELS: Record<BlockLayout, string> = {
  single: "Single",
  duo: "Duo",
  "main-aside": "Main and aside",
  trio: "Trio",
  "stack-2": "Stack 2",
  "stack-3": "Stack 3",
};

export function layoutChoiceIssue(
  layout: BlockLayout,
  width: BlockWidth,
  elementLabels: readonly string[],
): string | null {
  const choice = LAYOUTS[layout];
  if (elementLabels.length > choice.slots.length) {
    const stranded = elementLabels.slice(choice.slots.length);
    return `${LAYOUT_LABELS[layout]} has no room for ${stranded.join(", ")}. Move or remove ${stranded.length === 1 ? "it" : "them"} first.`;
  }
  if (WIDTH_COLUMNS[width] < choice.minimumColumns) {
    return `${LAYOUT_LABELS[layout]} needs ${widthLabelForColumns(choice.minimumColumns)} width. Widen this section first.`;
  }
  return null;
}

export function widthChoiceIssue(
  layout: BlockLayout,
  width: BlockWidth,
): string | null {
  const minimumColumns = LAYOUTS[layout].minimumColumns;
  if (WIDTH_COLUMNS[width] >= minimumColumns) return null;
  return `${LAYOUT_LABELS[layout]} needs ${widthLabelForColumns(minimumColumns)} width. Choose another layout before narrowing this section.`;
}

function widthLabelForColumns(columns: number): string {
  const width = (Object.entries(WIDTH_COLUMNS) as [BlockWidth, number][]).find(
    ([, value]) => value === columns,
  )?.[0];
  return width ? WIDTH_LABELS[width] : `${columns} columns`;
}

export type PackedBlock<T> = {
  block: T;
  columns: number;
};

export function packBlockRows<T extends { hidden: boolean; width: BlockWidth }>(
  blocks: readonly T[],
  options: { showHidden: boolean; narrow?: boolean },
): PackedBlock<T>[][] {
  const included = options.showHidden
    ? blocks
    : blocks.filter((block) => !block.hidden);
  if (options.narrow) {
    return included.map((block) => [{ block, columns: 12 }]);
  }

  const rows: PackedBlock<T>[][] = [];
  let row: PackedBlock<T>[] = [];
  let occupied = 0;

  function finishRow() {
    const last = row.at(-1);
    if (!last) return;
    row[row.length - 1] = {
      ...last,
      columns: last.columns + 12 - occupied,
    };
    rows.push(row);
    row = [];
    occupied = 0;
  }

  for (const block of included) {
    const columns = WIDTH_COLUMNS[block.width];
    if (occupied + columns > 12) finishRow();
    row.push({ block, columns });
    occupied += columns;
    if (occupied === 12) finishRow();
  }
  finishRow();
  return rows;
}

export const FULL_SCREEN_TYPES = [
  "entry_table",
  "image_set",
  "text_set",
  "dialogue_sample",
  "prompt_list",
  "variable_schema",
  "setting_group",
  "script_list",
] as const;

export function opensFullScreen(type: string): boolean {
  return (FULL_SCREEN_TYPES as readonly string[]).includes(type);
}

export const INLINE_ITEM_LIMIT = 8;

export function fitsInTheSheet(element: {
  type: string;
  content: unknown;
}): boolean {
  return (
    !opensFullScreen(element.type) ||
    contentItemCount(element) <= INLINE_ITEM_LIMIT
  );
}

export function contentItemCount(element: {
  type: string;
  content: unknown;
}): number {
  if (!element.content || typeof element.content !== "object") return 0;
  switch (element.type) {
    case "prose": {
      const text = (element.content as { text?: unknown }).text;
      return typeof text === "string" && text.trim() !== "" ? 1 : 0;
    }
    case "text_set": {
      const texts = (element.content as { texts?: unknown }).texts;
      return Array.isArray(texts) ? texts.length : 0;
    }
    case "dialogue_sample": {
      const turns = (element.content as { turns?: unknown }).turns;
      return Array.isArray(turns) ? turns.length : 0;
    }
    case "image_set": {
      const images = (element.content as { images?: unknown }).images;
      return Array.isArray(images) ? images.length : 0;
    }
    case "field_list":
      return collectionSize(element.content, "fields");
    case "entry_table":
      return collectionSize(element.content, "entries");
    case "link_list":
      return collectionSize(element.content, "links");
    case "prompt_list":
      return collectionSize(element.content, "fragments");
    case "variable_schema":
      return collectionSize(element.content, "variables");
    case "setting_group":
      return collectionSize(element.content, "settings");
    case "script_list":
      return collectionSize(element.content, "scripts");
    case "color_set":
      return nestedCollectionSize(element.content, "colors");
    case "stylesheet_set": {
      const content = element.content as {
        global?: unknown;
        stylesheets?: unknown;
      };
      const global =
        typeof content.global === "string" && content.global.trim() !== ""
          ? 1
          : 0;
      return global + collectionSize(element.content, "stylesheets");
    }
    case "record_list":
      return collectionSize(element.content, "records");
    default:
      return 0;
  }
}

function collectionSize(content: unknown, key: string): number {
  if (!content || typeof content !== "object") return 0;
  const collection = (content as Record<string, unknown>)[key];
  if (Array.isArray(collection)) return collection.length;
  if (collection && typeof collection === "object") {
    return Object.keys(collection).length;
  }
  return 0;
}

function nestedCollectionSize(content: unknown, key: string): number {
  if (!content || typeof content !== "object") return 0;
  const collection = (content as Record<string, unknown>)[key];
  if (Array.isArray(collection)) return collection.length;
  if (!collection || typeof collection !== "object") return 0;
  return Object.values(collection).reduce<number>((total, group) => {
    if (Array.isArray(group)) return total + group.length;
    if (group && typeof group === "object") {
      return total + Object.keys(group).length;
    }
    return total;
  }, 0);
}
