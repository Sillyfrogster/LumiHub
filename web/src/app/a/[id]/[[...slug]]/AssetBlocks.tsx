"use client";

import { PencilLine } from "lucide-react";
import { useEffect, useState } from "react";
import type { AssetBlock } from "@/lib/api/query";
import styles from "./AssetBlocks.module.css";
import { BlockSheet } from "./BlockSheet";
import { ElementBody } from "./ElementBody";

/** One row arrangement per layout preset the catalog can choose. */
const LAYOUT_CLASS: Record<AssetBlock["layout"], string> = {
  single: styles.single,
  duo: styles.duo,
  "main-aside": styles.mainAside,
  trio: styles.trio,
  "stack-2": styles.stack,
  "stack-3": styles.stack,
};

/**
 * The asset's content, in page order. A reader is shown the blocks that carry
 * something. The owner is shown every block the kind asks for, because an
 * empty required block is what says what is still theirs to write.
 */
export function AssetBlocks({
  assetId,
  blocks,
  isOwner,
}: {
  assetId: string;
  blocks: AssetBlock[];
  isOwner: boolean;
}) {
  const [currentBlocks, setCurrentBlocks] = useState(blocks);
  const [editing, setEditing] = useState<string | null>(null);

  useEffect(() => setCurrentBlocks(blocks), [blocks]);

  const shown = isOwner
    ? currentBlocks
    : currentBlocks.filter((block) => !block.hidden && !block.isEmpty);
  if (shown.length === 0) return null;

  const editedBlock = currentBlocks.find((block) => block.id === editing);

  return (
    <>
      <div className={styles.column}>
        {shown.map((block) => (
          <article
            id={`block-${block.id}`}
            key={block.id}
            className={styles.block}
          >
            <header className={styles.header}>
              <div className={styles.heading}>
                <h2 className={styles.title}>{block.title}</h2>
                {isOwner && block.required ? (
                  <span>{block.hideable ? "Required" : "Always shown"}</span>
                ) : null}
              </div>
              {isOwner ? (
                <button
                  type="button"
                  className={styles.edit}
                  aria-label={`Edit ${block.title}`}
                  onClick={() => setEditing(block.id)}
                >
                  <PencilLine size={15} aria-hidden="true" />
                  Edit section
                </button>
              ) : null}
            </header>
            <div className={LAYOUT_CLASS[block.layout]}>
              {block.elements.map((element) => (
                <ElementBody
                  key={element.id}
                  element={element}
                  isOwner={isOwner}
                />
              ))}
            </div>
          </article>
        ))}
      </div>
      {editedBlock ? (
        <BlockSheet
          assetId={assetId}
          block={editedBlock}
          onDismiss={() => setEditing(null)}
          onSaved={(saved) =>
            setCurrentBlocks((current) =>
              current.map((block) => (block.id === saved.id ? saved : block)),
            )
          }
        />
      ) : null}
    </>
  );
}
