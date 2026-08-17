import { QueryClient } from "@tanstack/react-query";
import { api } from "./client";
import type { components, paths } from "./schema";

export type AssetDetail = components["schemas"]["AssetDetail"];
export type AssetImage = components["schemas"]["AssetImage"];
export type AssetBlock = components["schemas"]["AssetBlock"];
export type AssetElement = components["schemas"]["AssetElement"];
export type AssetTag = components["schemas"]["AssetTag"];
export type Profile = components["schemas"]["Profile"];
export type BrowseAsset = components["schemas"]["BrowseAsset"];
export type BrowsePage = components["schemas"]["AssetList"];
export type BrowseCursor = components["schemas"]["BrowseCursor"];
export type DeletedAsset = components["schemas"]["DeletedAsset"];
export type BrowseKind = BrowseAsset["kind"];
export type NsfwVisibility =
  components["schemas"]["NsfwVisibilityRequest"]["visibility"];

type AssetQuery = NonNullable<
  paths["/v1/assets"]["get"]["parameters"]["query"]
>;

export type BrowseFilters = Pick<
  AssetQuery,
  "kind" | "platform" | "q" | "facet"
>;

export type AssetListParams = BrowseFilters &
  Pick<AssetQuery, "creator" | "limit" | "before" | "beforeId" | "nsfw">;

/** A fresh client every call. A shared one would leak one visitor's cache into another's page. */
export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { staleTime: 30_000, gcTime: 10 * 60_000 },
    },
  });
}

export const assetKeys = {
  all: ["assets"] as const,
  list: (
    filters: BrowseFilters,
    visibility?: NsfwVisibility,
    creator?: string,
  ) => ["assets", "list", creator, filters, visibility] as const,
};

export async function fetchProfile(handle: string): Promise<Profile | null> {
  const { data, error } = await api.GET("/v1/profiles/{handle}", {
    params: { path: { handle } },
  });
  if (error || !data) return null;
  return data;
}

export async function fetchAssets(
  params: AssetListParams,
  cookie?: string,
): Promise<BrowsePage> {
  const { data, error } = await api.GET("/v1/assets", {
    params: { query: params },
    headers: cookie ? { cookie } : undefined,
  });
  if (error) throw new Error("Could not load the collection");
  return data;
}

export async function fetchDeletedAssets(
  handle: string,
  cookie: string,
): Promise<DeletedAsset[] | null> {
  const { data, error } = await api.GET("/v1/profiles/{handle}/deleted", {
    params: { path: { handle } },
    headers: { cookie },
  });
  if (error || !data) return null;
  return data.items;
}

/**
 * One asset by id. A withheld, deleted or never-existed asset comes back null,
 * because the API answers all three the same way.
 */
export async function fetchAsset(
  id: string,
  cookie?: string,
): Promise<AssetDetail | null> {
  const { data, error } = await api.GET("/v1/assets/{id}", {
    params: { path: { id } },
    headers: cookie ? { cookie } : undefined,
  });
  if (error || !data) return null;
  return data;
}

/**
 * Starts an asset from nothing. The kind is asked for once, here, so the page
 * that comes back is already the right shape.
 */
export async function startAsset(kind: string): Promise<AssetDetail> {
  const { data, error } = await api.POST("/v1/assets", {
    body: { kind },
  });
  // The same route also accepts an upload, which answers with an ingest
  // operation, so the answer is narrowed to the page a start comes back with.
  if (error || !data || !("blocks" in data)) {
    throw new Error("Could not start the asset");
  }
  return data;
}

export async function saveNsfwVisibility(visibility: NsfwVisibility) {
  const { error } = await api.PUT("/v1/account/nsfw-visibility", {
    body: { visibility },
  });
  if (error) throw new Error("Could not save the content preference");
}

export async function saveAssetDiscovery(
  id: string,
  discovery: AssetDetail["discovery"],
) {
  const { error } = await api.PUT("/v1/assets/{id}/discovery", {
    params: { path: { id } },
    body: { discovery },
  });
  if (error) throw new Error("Could not save discovery");
}

export async function withholdAsset(id: string, reason: string) {
  const { error } = await api.PUT("/v1/assets/{id}/withhold", {
    params: { path: { id } },
    body: { reason },
  });
  if (error) throw new Error("Could not withhold the asset");
}

export async function deleteAsset(id: string) {
  const { error } = await api.DELETE("/v1/assets/{id}", {
    params: { path: { id } },
  });
  if (error) throw new Error("Could not delete the asset");
}

export async function restoreAsset(id: string) {
  const { error } = await api.POST("/v1/assets/{id}/restore", {
    params: { path: { id } },
  });
  if (error) throw new Error("Could not restore the asset");
}
