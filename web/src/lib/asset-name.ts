/**
 * What a page calls an asset before its creator has named it. A heading with
 * nothing in it reads as a broken page rather than an unfinished one.
 */
export function assetDisplayName(name: string): string {
  return name.trim() === "" ? "Untitled" : name;
}
