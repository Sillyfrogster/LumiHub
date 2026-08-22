type LabelledElement = {
  role?: string | null;
  label?: string;
};

type LabellingBlock = {
  title?: string;
  /** How many elements the block renders, which is not always how many it holds. */
  elements: number;
};

/**
 * The label an element carries above its content, or nothing. A label is worth
 * rendering when it tells one element in a block from another and says
 * something the block's title has not already said.
 */
export function elementLabel(
  element: LabelledElement,
  block: LabellingBlock,
): string | null {
  const label = element.label?.trim() ?? "";
  if (!element.role || label === "") return null;
  if (block.elements <= 1) return null;
  return labelRepeatsTitle(label, block.title) ? null : label;
}

function labelRepeatsTitle(
  label: string,
  blockTitle: string | undefined,
): boolean {
  const words = sameWords(label);
  const titleWords = sameWords(blockTitle ?? "");
  if (words.length === 0 || titleWords.length === 0) return false;
  return containsRun(titleWords, words);
}

/** The words of a phrase, stripped of the differences a reader does not hear. */
function sameWords(phrase: string): string[] {
  return phrase
    .toLocaleLowerCase()
    .replaceAll(/[^\p{Letter}\p{Number}\s]/gu, "")
    .split(/\s+/)
    .filter((word) => word !== "")
    .map(singular);
}

function singular(word: string): string {
  if (word.endsWith("ies")) return `${word.slice(0, -3)}y`;
  return word.endsWith("s") ? word.slice(0, -1) : word;
}

function containsRun(haystack: string[], run: string[]): boolean {
  for (let start = 0; start + run.length <= haystack.length; start += 1) {
    if (run.every((word, offset) => haystack[start + offset] === word)) {
      return true;
    }
  }
  return false;
}
