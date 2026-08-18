"use client";

import { PencilLine } from "lucide-react";
import { type CSSProperties, useEffect, useState } from "react";
import {
  type AssetBlock,
  type SaveAssetBlockRequest,
  saveAssetBlock,
} from "@/lib/api/query";
import { packBlockRows } from "@/lib/page-arrangement";
import { WidthPicker } from "./ArrangementPickers";
import styles from "./AssetBlocks.module.css";
import { BlockSheet } from "./BlockSheet";
import { ElementBody } from "./ElementBody";

/** One row arrangement per layout preset the catalog can choose. */
const LAYOUT_CLASS: Record<AssetBlock["layout"], string> = {
  single: styles.single,
  duo: styles.duo,
  "main-aside": styles.mainAside,
  trio: styles.trio,
  "stack-2": styles.stack2,
  "stack-3": styles.stack3,
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
  const [savingWidth, setSavingWidth] = useState<string | null>(null);
  const [arrangementMessage, setArrangementMessage] = useState("");

  useEffect(() => setCurrentBlocks(blocks), [blocks]);

  const packable = isOwner
    ? currentBlocks
    : currentBlocks.filter((block) => !block.isEmpty);
  const rows = packBlockRows(packable, { showHidden: isOwner });
  if (rows.length === 0) return null;

  const editedBlock = currentBlocks.find((block) => block.id === editing);

  return (
    <>
      {isOwner ? (
        <p className={styles.narrowNote}>
          Section widths arrange the desktop page. On this screen every section
          fills the width, and no content is lost.
        </p>
      ) : null}
      {arrangementMessage ? (
        <p className={styles.arrangementMessage} role="alert">
          {arrangementMessage}
        </p>
      ) : null}
      <div className={styles.rows}>
        {rows.map((row) => (
          <div
            className={styles.row}
            key={row.map((item) => item.block.id).join(":")}
          >
            {row.map(({ block, columns }) => (
              <article
                id={`block-${block.id}`}
                key={block.id}
                className={styles.block}
                style={{ "--block-columns": columns } as CSSProperties}
              >
                <header className={styles.header}>
                  <div className={styles.heading}>
                    <h2 className={styles.title}>{block.title}</h2>
                    {isOwner && block.required ? (
                      <span>
                        {block.hideable ? "Required" : "Always shown"}
                      </span>
                    ) : null}
                  </div>
                  {isOwner ? (
                    <div className={styles.controls}>
                      <WidthPicker
                        width={block.width}
                        layout={block.layout}
                        pending={savingWidth === block.id}
                        onIssue={setArrangementMessage}
                        onSelect={async (width) => {
                          if (savingWidth) return;
                          setSavingWidth(block.id);
                          setArrangementMessage("");
                          try {
                            const saved = await saveAssetBlock(
                              assetId,
                              block.id,
                              blockSaveRequest(block, width),
                            );
                            setCurrentBlocks((current) =>
                              current.map((item) =>
                                item.id === saved.id ? saved : item,
                              ),
                            );
                          } catch (error) {
                            setArrangementMessage(
                              error instanceof Error
                                ? error.message
                                : "The width could not be saved. Try again.",
                            );
                          } finally {
                            setSavingWidth(null);
                          }
                        }}
                      />
                      <button
                        type="button"
                        className={styles.edit}
                        aria-label={`Edit ${block.title}`}
                        onClick={() => setEditing(block.id)}
                      >
                        <PencilLine size={15} aria-hidden="true" />
                        <span>Edit section</span>
                      </button>
                    </div>
                  ) : null}
                </header>
                <div className={LAYOUT_CLASS[block.layout]}>
                  {block.elements.map((element) => (
                    <div style={{ gridArea: element.slot }} key={element.id}>
                      <ElementBody element={element} isOwner={isOwner} />
                    </div>
                  ))}
                </div>
              </article>
            ))}
          </div>
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

function blockSaveRequest(
  block: AssetBlock,
  width: AssetBlock["width"],
): SaveAssetBlockRequest {
  return {
    title: block.titleIsDefault ? null : block.title,
    layout: block.layout,
    width,
    elements: block.elements.map((element) => ({
      id: element.id,
      type: element.type,
      role: element.role,
      slot: element.slot,
      display: element.display,
      content: element.content,
    })),
  };
}
