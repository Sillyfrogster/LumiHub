import type { BrowseKind, StartAssetApp } from "./api/query";

export const KIND_LABELS: Record<BrowseKind, string> = {
  character: "Character",
  lorebook: "Lorebook",
  preset: "Preset",
  theme: "Theme",
};

export const BUILDABLE_KINDS: BrowseKind[] = [
  "character",
  "lorebook",
  "preset",
];

export const KINDS_ASKING_FOR_AN_APP: BrowseKind[] = ["preset"];

export const APP_CHOICES: { value: StartAssetApp; label: string }[] = [
  { value: "sillytavern", label: "SillyTavern" },
  { value: "lumiverse", label: "Lumiverse" },
];
