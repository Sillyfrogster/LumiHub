import { QueryClient } from "@tanstack/react-query";
import { type Asset, api } from "./client";

export type AssetListParams = {
  kind?: string;
  platform?: string;
  tag?: string[];
  facet?: string[];
  limit?: number;
  before?: string;
  beforeId?: string;
};

/** A fresh client every call. A shared one would leak one visitor's cache into another's page. */
export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { staleTime: 30_000 } },
  });
}

export const assetKeys = {
  list: (params: AssetListParams) => ["assets", "list", params] as const,
};

export async function fetchAssets(params: AssetListParams): Promise<Asset[]> {
  const { data, error } = await api.GET("/v1/assets", {
    params: { query: params },
  });
  if (error) throw new Error("Could not load assets");
  return data.items;
}
