import type { CSSProperties } from "react";
import type { BrowseKind } from "@/lib/api/query";
import { emptyPageInvitation } from "@/lib/empty-page-invitation";
import { KIND_LABELS } from "@/lib/kinds";
import { quietPageArtVariables } from "@/lib/quiet-page-art";
import styles from "./QuietPage.module.css";

/** Where on the page a piece of artwork is standing. */
export type ArtPlacement = "beside" | "inRow" | "atFoot";

const PLACEMENT_CLASS: Record<ArtPlacement, string> = {
  beside: styles.beside,
  inRow: styles.inRow,
  atFoot: styles.atFoot,
};

/**
 * The artwork that holds the space a page's content would have taken. It is
 * masked into the ground rather than framed, and it runs off the right edge.
 */
export function QuietPageArt({
  kind,
  placement,
  style,
}: {
  kind: BrowseKind;
  placement: ArtPlacement;
  style?: CSSProperties;
}) {
  return (
    <div
      aria-hidden="true"
      data-measurement-ignore
      className={`${styles.art} ${PLACEMENT_CLASS[placement]}`}
      style={{ ...quietPageArtVariables(kind), ...style } as CSSProperties}
    />
  );
}

/**
 * A reader's page for an asset that displays nothing. The creator may have
 * written no more than the upload, or hidden all of it on purpose. Either way
 * the file is intact, which is the one thing worth saying.
 */
export function EmptyPage({ kind }: { kind: BrowseKind }) {
  const label = KIND_LABELS[kind].toLowerCase();

  return (
    <QuietComposition kind={kind} heading="Nothing is shown here">
      The creator has put none of this {label} on the page. What the file holds
      is kept, and every download carries it.
    </QuietComposition>
  );
}

/**
 * The owner's page for an asset that holds nothing yet. One invitation naming
 * the blocks this kind is built around, in place of a marker on every element.
 */
export function EmptyPageInvitation({
  kind,
  coreBlocks,
  canAdd,
}: {
  kind: BrowseKind;
  coreBlocks: readonly string[];
  canAdd: boolean;
}) {
  return (
    <QuietComposition kind={kind} heading="Nothing on this page yet" compact>
      {emptyPageInvitation({
        coreBlocks,
        canAdd,
        kindLabel: KIND_LABELS[kind].toLowerCase(),
      })}
    </QuietComposition>
  );
}

/**
 * Words in a narrow column with the kind's artwork beside them. An owner is at
 * work on the page below, so their band is the shorter of the two.
 */
function QuietComposition({
  kind,
  heading,
  compact = false,
  children,
}: {
  kind: BrowseKind;
  heading: string;
  compact?: boolean;
  children: React.ReactNode;
}) {
  return (
    <section
      className={compact ? `${styles.quiet} ${styles.compact}` : styles.quiet}
      aria-labelledby="quiet-page-heading"
    >
      <div className={styles.words}>
        <h2 id="quiet-page-heading">{heading}</h2>
        <p>{children}</p>
      </div>
      <QuietPageArt kind={kind} placement="beside" />
    </section>
  );
}
