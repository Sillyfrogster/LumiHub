import styles from "./CreatorMark.module.css";

/** Spreads handles across the tonal steps and mark angles without storing a value. */
function shadeOf(handle: string) {
  let hash = 0;
  for (const character of handle) {
    hash = (hash * 31 + (character.codePointAt(0) ?? 0)) % 100000;
  }
  return hash;
}

export function CreatorMark({
  handle,
  compact = false,
}: {
  handle: string;
  compact?: boolean;
}) {
  const hash = shadeOf(handle);

  return (
    <span
      className={styles.mark}
      data-tone={hash % 4}
      data-compact={compact || undefined}
      style={{ "--mark-angle": `${hash % 90}deg` } as React.CSSProperties}
      aria-hidden="true"
    >
      <span className={styles.figure} />
      <span className={styles.initial}>{handle.slice(0, 1).toUpperCase()}</span>
    </span>
  );
}
