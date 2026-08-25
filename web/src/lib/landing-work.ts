import type { BrowseAsset } from "./api/query";

/** Orders distinct catalog work with real covers first */
export function withCoversFirst(assets: BrowseAsset[]): BrowseAsset[] {
  const seen = new Set<string>();
  const distinct = assets.filter((asset) => {
    const key = asset.name || asset.id;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });

  return [
    ...distinct.filter((asset) => asset.cover),
    ...distinct.filter((asset) => !asset.cover),
  ];
}
