import { BookOpen, Palette, SlidersHorizontal, UserRound } from "lucide-react";
import type { BrowseKind } from "@/lib/api/query";
import { KIND_LABELS } from "@/lib/kinds";
import styles from "./DefaultCover.module.css";

const ICONS = {
  character: UserRound,
  lorebook: BookOpen,
  preset: SlidersHorizontal,
  theme: Palette,
} as const;

export function DefaultCover({
  kind,
  compact = false,
}: {
  kind: BrowseKind;
  compact?: boolean;
}) {
  const Icon = ICONS[kind];

  return (
    <span
      className={styles.cover}
      data-kind={kind}
      data-compact={compact || undefined}
      aria-hidden="true"
    >
      <span className={styles.arch}>
        <Icon size={compact ? 22 : 38} strokeWidth={1.25} />
      </span>
      {compact ? null : (
        <span className={styles.label}>{KIND_LABELS[kind]}</span>
      )}
    </span>
  );
}
