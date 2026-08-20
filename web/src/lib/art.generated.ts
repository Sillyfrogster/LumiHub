/** Generated file. Do not edit by hand. */
import type { StaticImageData } from "next/image";
import balustrade from "@/assets/art/balustrade.webp";
import birds from "@/assets/art/birds.webp";
import city from "@/assets/art/city.webp";
import cornerBr from "@/assets/art/corner-br.webp";
import cornerTl from "@/assets/art/corner-tl.webp";
import dividerStar from "@/assets/art/divider-star.webp";
import heroLumi from "@/assets/art/full/hero-lumi.webp";
import ruleFlower from "@/assets/art/rule-flower.webp";
import sparkles from "@/assets/art/sparkles.webp";
import sprig from "@/assets/art/sprig.webp";
import wash from "@/assets/art/wash.webp";

export const ART = {
  balustrade: balustrade,
  birds: birds,
  city: city,
  "corner-br": cornerBr,
  "corner-tl": cornerTl,
  "divider-star": dividerStar,
  "hero-lumi": heroLumi,
  "rule-flower": ruleFlower,
  sparkles: sparkles,
  sprig: sprig,
  wash: wash,
} satisfies Record<string, StaticImageData>;

export type ArtName = keyof typeof ART;

export const ART_NAMES = Object.keys(ART) as ArtName[];
