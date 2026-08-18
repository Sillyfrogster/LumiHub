export const WIDTH_COLUMNS = {
  full: 12,
  two_thirds: 8,
  half: 6,
  third: 4,
} as const;

export type BlockWidth = keyof typeof WIDTH_COLUMNS;

export const LAYOUTS = {
  single: { slots: ["main"], minimumColumns: 4 },
  duo: { slots: ["left", "right"], minimumColumns: 8 },
  "main-aside": { slots: ["main", "aside"], minimumColumns: 8 },
  trio: { slots: ["left", "middle", "right"], minimumColumns: 12 },
  "stack-2": { slots: ["top", "bottom"], minimumColumns: 4 },
  "stack-3": { slots: ["top", "middle", "bottom"], minimumColumns: 4 },
} as const;

export type BlockLayout = keyof typeof LAYOUTS;

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
    default:
      return 0;
  }
}
