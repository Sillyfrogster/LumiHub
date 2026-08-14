export type CreationKind = "character" | "lorebook" | "preset";

export interface Creation {
  id: string;
  title: string;
  author: string;
  kind: CreationKind;
  cover?: string;
  adult?: boolean;
}

export const KIND_LABEL: Record<CreationKind, string> = {
  character: "CHARACTER",
  lorebook: "LOREBOOK",
  preset: "PRESET",
};
