import type { CreationKind } from "@/types/creation";

export interface Frontend {
  name: string;
  mark: string;
}

export const FRONTENDS: Frontend[] = [
  { name: "Lumiverse", mark: "L" },
  { name: "SillyTavern", mark: "S" },
  { name: "Risu", mark: "R" },
  { name: "Chub", mark: "C" },
];

export interface CreationType {
  kind: CreationKind;
  name: string;
  count: string;
}

export const CREATION_TYPES: CreationType[] = [
  { kind: "character", name: "Characters", count: "14.2K" },
  { kind: "lorebook", name: "Lorebooks", count: "2.1K" },
  { kind: "preset", name: "Presets", count: "3.8K" },
];

export const SITE_STATS = [
  { value: "40K+", label: "Active creators" },
  { value: "120K+", label: "Public creations" },
  { value: "∞", label: "Possibilities" },
];

export const CREATOR_BENEFITS = [
  {
    icon: "upload",
    title: "Publish with ease",
    body: "Upload once and reach any frontend. Simple tools, powerful impact.",
  },
  {
    icon: "audience",
    title: "Grow your audience",
    body: "Connect with fans, build credibility, and grow your creative presence.",
  },
  {
    icon: "reward",
    title: "Earn & get recognized",
    body: "Monetize your work, gain support, and earn creator rewards.",
  },
] as const;

export const SPOTLIGHT = {
  quote:
    "LumiHub is the best thing to happen to the roleplay community. I've found so many incredible creators and resources that make my worlds come alive.",
  name: "Haruki S.",
  role: "Creator & Worldbuilder",
};
