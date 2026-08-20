import { QueryClient } from "@tanstack/react-query";
import { api } from "./client";
import type { components, paths } from "./schema";

export type AssetDetail = components["schemas"]["AssetDetail"];
export type AssetImage = components["schemas"]["AssetImage"];
export type AssetBlock = components["schemas"]["AssetBlock"];
export type AssetElement = components["schemas"]["AssetElement"];
export type SaveAssetBlockRequest =
  components["schemas"]["SaveAssetBlockRequest"];
export type ArrangeAssetBlocksRequest =
  components["schemas"]["ArrangeAssetBlocksRequest"];
export type EntryTableContent = components["schemas"]["EntryTableContent"];
export type LorebookEntry = EntryTableContent["entries"][number];
export type PromptListContent = components["schemas"]["PromptListContent"];
export type PromptGroup = PromptListContent["groups"][number];
export type PromptFragment = PromptListContent["fragments"][number];
export type SettingGroupContent = components["schemas"]["SettingGroupContent"];
export type PresetSetting = SettingGroupContent["settings"][number];
export type VariableSchemaContent =
  components["schemas"]["VariableSchemaContent"];
export type PresetVariable = VariableSchemaContent["variables"][number];
export type ScriptListContent = components["schemas"]["ScriptListContent"];
export type RegexScript = ScriptListContent["scripts"][number];
export type TypedValue = components["schemas"]["TypedValue"];
export type AddableSection = components["schemas"]["AddableSection"];
export type ElementType = components["schemas"]["ElementType"];
export type AssetTag = components["schemas"]["AssetTag"];
export type ReadinessItem = components["schemas"]["ReadinessItem"];
export type PreservedNamespace = components["schemas"]["PreservedNamespace"];
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

// The API intentionally makes withheld, deleted, and missing assets identical.
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

export type StartAssetApp = NonNullable<
  components["schemas"]["StartAssetRequest"]["app"]
>;

export async function startAsset(
  kind: string,
  app?: StartAssetApp,
): Promise<AssetDetail> {
  const { data, error } = await api.POST("/v1/assets", {
    body: app ? { kind, app } : { kind },
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

/** Saves one builder section without changing any other section on the page. */
export async function saveAssetBlock(
  assetId: string,
  blockId: string,
  block: SaveAssetBlockRequest,
): Promise<AssetBlock | null> {
  const { data, error, response } = await api.PUT(
    "/v1/assets/{id}/blocks/{blockId}",
    {
      params: { path: { id: assetId, blockId } },
      body: block,
    },
  );
  if (response.status === 204) return null;
  if (error || !data) {
    const detail = error as { error?: unknown } | undefined;
    const message =
      typeof detail?.error === "string"
        ? detail.error.replace(/^invalid block:\s*/i, "")
        : "The section could not be saved. Try again.";
    throw new Error(message);
  }
  return data;
}

/** Adds one section to the foot of the page, holding the element chosen for it. */
export async function addAssetBlock(
  assetId: string,
  definition: string,
  elementType: ElementType,
): Promise<AssetBlock> {
  const { data, error } = await api.POST("/v1/assets/{id}/blocks", {
    params: { path: { id: assetId } },
    body: { definition, elementType },
  });
  if (error || !data) {
    const detail = error as { error?: unknown } | undefined;
    throw new Error(
      typeof detail?.error === "string"
        ? detail.error.replace(/^invalid block:\s*/i, "")
        : "The section could not be added. Try again.",
    );
  }
  return data;
}

// Media is stored first, then linked when its section is saved.
export async function addAssetImage(
  assetId: string,
  file: File,
  role: "expression" | "gallery",
): Promise<string> {
  const body = new FormData();
  body.append("metadata", JSON.stringify({ role }));
  body.append("file", file, file.name);
  const response = await fetch(`/api/v1/assets/${assetId}/media`, {
    method: "POST",
    credentials: "same-origin",
    body,
  });
  if (!response.ok) {
    throw new Error(
      response.status === 413
        ? "That image is larger than Illarin accepts."
        : "The image could not be added. Try again.",
    );
  }
  const added = (await response.json()) as { id?: unknown };
  if (typeof added.id !== "string") {
    throw new Error("The image could not be added. Try again.");
  }
  return added.id;
}

/** Saves the full page outline as one arrangement. */
export async function arrangeAssetBlocks(
  assetId: string,
  arrangement: ArrangeAssetBlocksRequest,
): Promise<AssetBlock[]> {
  const { data, error } = await api.PUT("/v1/assets/{id}/blocks", {
    params: { path: { id: assetId } },
    body: arrangement,
  });
  if (error || !data) {
    const detail = error as { error?: unknown } | undefined;
    throw new Error(
      typeof detail?.error === "string"
        ? detail.error.replace(/^invalid block:\s*/i, "")
        : "The section order could not be saved. Try again.",
    );
  }
  return data;
}

/** Removes one optional section and all of the elements it holds. */
export async function removeAssetBlock(assetId: string, blockId: string) {
  const { error } = await api.DELETE("/v1/assets/{id}/blocks/{blockId}", {
    params: { path: { id: assetId, blockId } },
  });
  if (error) {
    const detail = error as { error?: unknown } | undefined;
    throw new Error(
      typeof detail?.error === "string"
        ? detail.error.replace(/^invalid block:\s*/i, "")
        : "The section could not be removed. Try again.",
    );
  }
}

/** Moves a section's unpinned content, then removes the emptied section. */
export async function moveAssetBlockContent(
  assetId: string,
  blockId: string,
  destinationBlockId: string,
): Promise<AssetBlock[]> {
  const { data, error } = await api.POST(
    "/v1/assets/{id}/blocks/{blockId}/move-and-remove",
    {
      params: { path: { id: assetId, blockId } },
      body: { destinationBlockId },
    },
  );
  if (error || !data) {
    const detail = error as { error?: unknown } | undefined;
    throw new Error(
      typeof detail?.error === "string"
        ? detail.error.replace(/^invalid block:\s*/i, "")
        : "The content could not be moved. Try again.",
    );
  }
  return data;
}

// A null adult-content answer is allowed only while the asset is a draft.
export async function saveAssetIdentity(
  id: string,
  identity: { name: string; isNsfw: boolean | null },
) {
  const { error } = await api.PUT("/v1/assets/{id}/identity", {
    params: { path: { id } },
    body: identity,
  });
  if (error) {
    const detail = error as { error?: unknown } | undefined;
    throw new Error(
      typeof detail?.error === "string"
        ? detail.error
        : "The details could not be saved. Try again.",
    );
  }
}

/**
 * Publishes a draft, or comes back with what publication is still waiting on.
 */
export async function publishAsset(
  id: string,
): Promise<
  | { published: true }
  | { published: false; error: string; readiness?: ReadinessItem[] }
> {
  const { data, error } = await api.POST("/v1/assets/{id}/publish", {
    params: { path: { id } },
  });
  if (data) return { published: true };
  const refusal = error as
    | { error?: unknown; readiness?: ReadinessItem[] }
    | undefined;
  return {
    published: false,
    error:
      typeof refusal?.error === "string"
        ? refusal.error
        : "The asset could not be published. Try again.",
    readiness: refusal?.readiness,
  };
}

// Preserved namespaces belong to the source file and are owner-only.
export async function fetchPreservedNamespaces(
  id: string,
): Promise<PreservedNamespace[]> {
  const { data } = await api.GET("/v1/assets/{id}/preserved", {
    params: { path: { id } },
  });
  return data ?? [];
}

/** Deletes one namespace and everything under it, for good. */
export async function deletePreservedNamespace(id: string, namespace: string) {
  const { error } = await api.DELETE("/v1/assets/{id}/preserved/{namespace}", {
    params: { path: { id, namespace } },
  });
  if (error) {
    const detail = error as { error?: unknown } | undefined;
    throw new Error(
      typeof detail?.error === "string"
        ? detail.error
        : "That data could not be deleted. Try again.",
    );
  }
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
