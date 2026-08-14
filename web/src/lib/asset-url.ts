const SLUG_LIMIT = 60;

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
