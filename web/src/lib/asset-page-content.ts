type PageElement = {
  isEmpty: boolean;
  role?: string | null;
};

const MODEL_DISCLOSURE_ROLES = new Set([
  "system_prompt",
  "post_history_instructions",
]);

function belongsInModelDisclosure(element: PageElement): boolean {
  return MODEL_DISCLOSURE_ROLES.has(element.role ?? "");
}

export function splitAssetPageContent<
  TElement extends PageElement,
  TBlock extends { hidden: boolean },
>(blocks: readonly (TBlock & { elements: TElement[] })[]) {
  const publicBlocks = blocks.map((block) => {
    const elements = block.elements.filter(
      (element) => !element.isEmpty && !belongsInModelDisclosure(element),
    );
    return { ...block, elements, empty: elements.length === 0 };
  });
  const modelContent: Array<{
    block: TBlock & { elements: TElement[] };
    element: TElement;
  }> = [];
  for (const block of blocks) {
    if (block.hidden) continue;
    for (const element of block.elements) {
      if (!element.isEmpty && belongsInModelDisclosure(element)) {
        modelContent.push({ block, element });
      }
    }
  }
  return { publicBlocks, modelContent };
}

/** Whether a reader's page draws this block. One that does not holds no columns. */
export function rendersOnThePage(block: {
  hidden: boolean;
  empty: boolean;
}): boolean {
  return !block.hidden && !block.empty;
}

/** How many blocks an invitation can name before it stops being a sentence. */
const INVITATION_BLOCK_LIMIT = 3;

type FillableBlock = {
  title: string;
  required: boolean;
  isEmpty: boolean;
};

/**
 * Whether no block on the asset has anything in it. A page that displays
 * nothing is a different question, because hidden content is still held and
 * still travels.
 */
export function assetHoldsNothing(
  blocks: readonly Pick<FillableBlock, "isEmpty">[],
): boolean {
  return blocks.every((block) => block.isEmpty);
}

/**
 * The blocks an invitation names. A kind's required blocks are its core. A
 * kind that requires none is invited to fill in whatever it was given.
 */
export function coreBlockTitles(
  blocks: readonly Pick<FillableBlock, "title" | "required">[],
): string[] {
  const required = blocks.filter((block) => block.required);
  const named = required.length > 0 ? required : blocks;
  return named.slice(0, INVITATION_BLOCK_LIMIT).map((block) => block.title);
}
