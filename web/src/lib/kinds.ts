import type { BrowseKind, StartAssetApp } from "./api/query";

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
export const BUILDABLE_KINDS: BrowseKind[] = [
  "character",
  "lorebook",
  "preset",
];

/**
 * The kinds whose settings have names only an app can give them. Creating one
 * asks which app it is for, once. The API refuses an unanswered one and
 * refuses an answer sent for any other kind, so this mirrors the same list.
 */
export const KINDS_ASKING_FOR_AN_APP: BrowseKind[] = ["preset"];

/**
 * The apps a preset can be built for, in the order they are offered. The
 * answer seeds the settings names and is stored nowhere.
 */
export const APP_CHOICES: { value: StartAssetApp; label: string }[] = [
  { value: "sillytavern", label: "SillyTavern" },
  { value: "lumiverse", label: "Lumiverse" },
];

/**
 * Category artwork for creations whose authors did not supply a preview.
 */
export const DEFAULT_COVERS: Record<BrowseKind, string> = {
  character: "/covers/cover-default-character-v2.webp",
  lorebook: "/covers/cover-default-lorebook-v2.webp",
  preset: "/covers/cover-default-preset-v2.webp",
  theme: "/covers/cover-default-theme-v2.webp",
};
