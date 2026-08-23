/**
 * What an owner is told on a page that holds nothing. One invitation naming
 * the blocks this kind is built around and the control that opens them, said
 * once at the top of the page rather than beside every empty label.
 */
export function emptyPageInvitation({
  coreBlocks,
  canAdd,
  kindLabel,
}: {
  /** The blocks worth naming, from `coreBlockTitles`. */
  coreBlocks: readonly string[];
  /** Whether this kind has blocks left to bring in. */
  canAdd: boolean;
  /** The kind in lower case, as a reader would say it. */
  kindLabel: string;
}): string {
  const named = namedInSentence(coreBlocks);

  if (named) {
    const fill = `Fill in ${named} to give the page something to show.`;
    return canAdd
      ? `${fill} Edit block opens it, and Add block brings in anything else a ${kindLabel} can hold.`
      : `${fill} Edit block opens it.`;
  }

  return canAdd
    ? `Add block brings in the first of what a ${kindLabel} can hold.`
    : `Illarin has no blocks for a ${kindLabel} yet. The file you uploaded is kept whole, and every download carries it.`;
}

/** How the invitation reads out the blocks it names. */
function namedInSentence(titles: readonly string[]): string {
  if (titles.length <= 1) return titles[0] ?? "";
  if (titles.length === 2) return titles.join(" and ");
  return `${titles.slice(0, -1).join(", ")} and ${titles.at(-1)}`;
}
