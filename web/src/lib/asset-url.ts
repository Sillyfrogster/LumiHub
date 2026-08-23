const SLUG_LIMIT = 60;

const ASSET_ID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Whether a path segment could be an asset id, so a lookup only runs when it could answer. */
export function isAssetId(segment: string): boolean {
  return ASSET_ID.test(segment);
}

/**
 * The decorative half of an asset's address. Nothing resolves by it, so a
 * rename cannot break a link and a collision costs nothing. A name that
 * normalizes to nothing has no slug.
 */
export function assetSlug(name: string): string {
  const normalized = name
    .normalize("NFKD")
    .replace(/\p{M}+/gu, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");

  if (normalized.length <= SLUG_LIMIT) return normalized;

  const capped = normalized.slice(0, SLUG_LIMIT);
  const boundary = capped.lastIndexOf("-");
  return boundary > 0 ? capped.slice(0, boundary) : capped;
}

/** The canonical address of an asset. */
export function assetHref(id: string, name: string): string {
  const slug = assetSlug(name);
  return slug ? `/a/${id}/${slug}` : `/a/${id}`;
}

/**
 * Where an address should send a reader, or null when they are already at the
 * canonical one. The redirect is temporary because a rename moves it.
 */
export function assetRedirect(
  visited: { id: string; slug?: string[] },
  asset: { id: string; name: string },
): string | null {
  const here = visited.slug?.length
    ? `/a/${visited.id}/${visited.slug.join("/")}`
    : `/a/${visited.id}`;
  const canonical = assetHref(asset.id, asset.name);
  return here === canonical ? null : canonical;
}
