import {
  BookOpen,
  type LucideIcon,
  Palette,
  SlidersHorizontal,
  UserRound,
} from "lucide-react";
import type { BrowseKind } from "@/lib/api/query";
import { KIND_LABELS } from "@/lib/kinds";
import styles from "./EmptyArtwork.module.css";

const KIND_ICONS: Record<BrowseKind, LucideIcon> = {
  character: UserRound,
  lorebook: BookOpen,
  preset: SlidersHorizontal,
  theme: Palette,
};

export function EmptyArtwork({
  kind,
  name,
  compact = false,
}: {
  kind: BrowseKind;
  name?: string;
  compact?: boolean;
}) {
  const Icon = KIND_ICONS[kind];

  return (
    <div className={styles.empty} data-compact={compact ? "" : undefined}>
      <Icon aria-hidden="true" />
      {name ? <strong>{name}</strong> : null}
      <span>No preview supplied</span>
      <small>{KIND_LABELS[kind]}</small>
    </div>
  );
}
