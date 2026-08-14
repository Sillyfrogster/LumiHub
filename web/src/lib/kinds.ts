import type { BrowseKind } from "./api/query";

export const KIND_LABELS: Record<BrowseKind, string> = {
  character: "Character",
  lorebook: "Lorebook",
  preset: "Preset",
  theme: "Theme",
};

/**
 * Category artwork for creations whose authors did not supply a preview.
 */
export const DEFAULT_COVERS: Record<BrowseKind, string> = {
  character: "/covers/cover-default-character-v2.webp",
  lorebook: "/covers/cover-default-lorebook-v2.webp",
  preset: "/covers/cover-default-preset-v2.webp",
  theme: "/covers/cover-default-theme-v2.webp",
};
