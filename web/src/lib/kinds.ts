import type { BrowseKind } from "./api/query";

export const KIND_LABELS: Record<BrowseKind, string> = {
  character: "Character",
  lorebook: "Lorebook",
  preset: "Preset",
  theme: "Theme",
};

/**
 * LumiHub's own cover art, used whenever a creator supplied none and whenever
 * theirs fails to load. A coverless asset keeps the ordinary layout.
 */
export const DEFAULT_COVERS: Record<BrowseKind, string> = {
  character: "/covers/cover-default-character-lumi.webp",
  lorebook: "/covers/cover-default-lorebook-lumi.webp",
  preset: "/covers/cover-default-preset-lumi.webp",
  theme: "/covers/cover-default-theme-lumi.webp",
};
