import { QueryClient } from "@tanstack/react-query";
import { api } from "./client";
import type { components, paths } from "./schema";

export type BrowseAsset = components["schemas"]["BrowseAsset"];
export type BrowsePage = components["schemas"]["AssetList"];
export type BrowseCursor = components["schemas"]["BrowseCursor"];
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
  Pick<AssetQuery, "limit" | "before" | "beforeId" | "nsfw">;

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
  list: (filters: BrowseFilters, visibility?: NsfwVisibility) =>
    ["assets", "list", filters, visibility] as const,
};

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

export async function saveNsfwVisibility(visibility: NsfwVisibility) {
  const { error } = await api.PUT("/v1/account/nsfw-visibility", {
    body: { visibility },
  });
  if (error) throw new Error("Could not save the content preference");
}
