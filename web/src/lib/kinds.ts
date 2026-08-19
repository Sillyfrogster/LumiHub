import type { BrowseKind } from "./api/query";

export const KIND_LABELS: Record<BrowseKind, string> = {
  character: "Character",
  lorebook: "Lorebook",
  preset: "Preset",
  theme: "Theme",
};

/**
 * The kinds Illarin can build from nothing. It mirrors the block catalogs in
 * the API, which refuse any other kind, and grows as each kind's catalog lands.
 */
export const BUILDABLE_KINDS: BrowseKind[] = ["character", "lorebook"];

/**
 * Category artwork for creations whose authors did not supply a preview.
 */
export const DEFAULT_COVERS: Record<BrowseKind, string> = {
  character: "/covers/cover-default-character-v2.webp",
  lorebook: "/covers/cover-default-lorebook-v2.webp",
  preset: "/covers/cover-default-preset-v2.webp",
  theme: "/covers/cover-default-theme-v2.webp",
};
