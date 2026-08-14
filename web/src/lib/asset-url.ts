export function assetHref(id: string, name: string) {
  const normalized = name
    .normalize("NFKD")
    .replace(/\p{M}+/gu, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");

  if (!normalized) return `/a/${id}`;
  if (normalized.length <= 60) return `/a/${id}/${normalized}`;

  const capped = normalized.slice(0, 60);
  const boundary = capped.lastIndexOf("-");
  const slug = boundary > 0 ? capped.slice(0, boundary) : capped;
  return `/a/${id}/${slug}`;
}
