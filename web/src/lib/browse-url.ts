import type { BrowseFilters, BrowseKind } from "./api/query";

const KINDS = new Set<BrowseKind>([
  "character",
  "lorebook",
  "preset",
  "theme",
  "pack",
]);

function first(value: string | string[] | undefined) {
  return Array.isArray(value) ? value[0] : value;
}

export function readBrowseFilters(
  values: Record<string, string | string[] | undefined>,
): BrowseFilters {
  const requestedKind = first(values.kind);
  const kind = KINDS.has(requestedKind as BrowseKind)
    ? (requestedKind as BrowseKind)
    : undefined;
  const q = first(values.q) || undefined;
  const platform = first(values.platform)?.trim() || undefined;
  const facets = Array.isArray(values.facet)
    ? values.facet
    : values.facet
      ? [values.facet]
      : undefined;

  return { kind, q, platform, facet: facets };
}

export function buildBrowseHref(filters: BrowseFilters, basePath = "/browse") {
  const params = new URLSearchParams();
  if (filters.kind) params.set("kind", filters.kind);
  if (filters.platform) params.set("platform", filters.platform);
  if (filters.q) params.set("q", filters.q);
  for (const facet of filters.facet ?? []) params.append("facet", facet);
  const query = params.toString();
  return query ? `${basePath}?${query}` : basePath;
}
