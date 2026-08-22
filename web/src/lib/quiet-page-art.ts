import type { StaticImageData } from "next/image";
import characterDark from "@/assets/art/full/illarin-quiet-page-character-dark-v1.webp";
import characterLight from "@/assets/art/full/illarin-quiet-page-character-light-v1.webp";
import lorebookDark from "@/assets/art/full/illarin-quiet-page-lorebook-dark-v1.webp";
import lorebookLight from "@/assets/art/full/illarin-quiet-page-lorebook-light-v1.webp";
import packDark from "@/assets/art/full/illarin-quiet-page-pack-dark-v1.webp";
import packLight from "@/assets/art/full/illarin-quiet-page-pack-light-v1.webp";
import presetDark from "@/assets/art/full/illarin-quiet-page-preset-dark-v1.webp";
import presetLight from "@/assets/art/full/illarin-quiet-page-preset-light-v1.webp";
import themeDark from "@/assets/art/full/illarin-quiet-page-theme-dark-v1.webp";
import themeLight from "@/assets/art/full/illarin-quiet-page-theme-light-v1.webp";
import type { BrowseKind } from "./api/query";

/**
 * The artwork a page shows when its creator has displayed nothing, or close to
 * nothing. Each kind has its own piece, and each piece is composed twice so
 * neither theme is the other one inverted.
 */
export type QuietPageArt = {
  light: StaticImageData;
  dark: StaticImageData;
};

/** Packs are not a kind Illarin builds yet, and their piece is ready for it. */
const QUIET_PAGE_ART: Record<BrowseKind | "pack", QuietPageArt> = {
  character: { light: characterLight, dark: characterDark },
  lorebook: { light: lorebookLight, dark: lorebookDark },
  preset: { light: presetLight, dark: presetDark },
  theme: { light: themeLight, dark: themeDark },
  pack: { light: packLight, dark: packDark },
};

export function quietPageArt(kind: BrowseKind | "pack"): QuietPageArt {
  return QUIET_PAGE_ART[kind];
}

/**
 * The two files as CSS urls, so one rule can pick the piece and another can
 * pick the theme.
 */
export function quietPageArtVariables(
  kind: BrowseKind | "pack",
): Record<string, string> {
  const art = quietPageArt(kind);
  return {
    "--quiet-art-light": `url(${art.light.src})`,
    "--quiet-art-dark": `url(${art.dark.src})`,
  };
}
