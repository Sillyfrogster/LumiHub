"use client";

import { Eye, ListTree, PencilLine, Undo2 } from "lucide-react";
import { type CSSProperties, useEffect, useState } from "react";
import {
  type AssetBlock,
  arrangeAssetBlocks,
  moveAssetBlockContent,
  removeAssetBlock,
  type SaveAssetBlockRequest,
  saveAssetBlock,
} from "@/lib/api/query";
import { packBlockRows } from "@/lib/page-arrangement";
import { WidthPicker } from "./ArrangementPickers";
import {
  ArrangeSections,
  moveContentDestinations,
  RemoveSectionDialog,
} from "./ArrangeSections";
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
  const [arranging, setArranging] = useState(false);
  const [readerView, setReaderView] = useState(false);
  const [removing, setRemoving] = useState<AssetBlock | null>(null);
  const [blockActionPending, setBlockActionPending] = useState(false);

  useEffect(() => setCurrentBlocks(blocks), [blocks]);

  const editingVisible = isOwner && !readerView;
  const packable = editingVisible
    ? currentBlocks
    : currentBlocks.filter((block) => !block.isEmpty);
  const rows = packBlockRows(packable, { showHidden: editingVisible });

  const editedBlock = currentBlocks.find((block) => block.id === editing);

  async function runBlockAction(action: () => Promise<void>) {
    if (blockActionPending) return;
    setBlockActionPending(true);
    setArrangementMessage("");
    try {
      await action();
    } catch (error) {
      setArrangementMessage(
        error instanceof Error
          ? error.message
          : "The section could not be changed. Try again.",
      );
    } finally {
      setBlockActionPending(false);
    }
  }

  return (
    <>
      {isOwner ? (
        <div className={styles.ownerToolbar}>
          {readerView ? (
            <>
              <span>
                <Eye size={16} aria-hidden="true" /> Reader’s view
              </span>
              <button type="button" onClick={() => setReaderView(false)}>
                <Undo2 size={16} aria-hidden="true" />
                Return to editing
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                aria-expanded={arranging}
                onClick={() => {
                  setEditing(null);
                  setArranging((current) => !current);
                }}
              >
                <ListTree size={17} aria-hidden="true" />
                {arranging ? "Close outline" : "Arrange sections"}
              </button>
              <button
                type="button"
                onClick={() => {
                  setEditing(null);
                  setArranging(false);
                  setReaderView(true);
                }}
              >
                <Eye size={17} aria-hidden="true" />
                Reader’s view
              </button>
            </>
          )}
        </div>
      ) : null}

      {arranging && editingVisible ? (
        <ArrangeSections
          assetId={assetId}
          blocks={currentBlocks}
          onChange={setCurrentBlocks}
          onClose={() => setArranging(false)}
        />
      ) : (
        <>
          {editingVisible ? (
            <p className={styles.narrowNote}>
              Section widths arrange the desktop page. On this screen every
              section fills the width, and no content is lost.
            </p>
          ) : null}
          {arrangementMessage ? (
            <p className={styles.arrangementMessage} role="alert">
              {arrangementMessage}
            </p>
          ) : null}
          {rows.length === 0 ? null : (
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
                      data-hidden={
                        editingVisible && block.hidden ? true : undefined
                      }
                      style={{ "--block-columns": columns } as CSSProperties}
                    >
                      <header className={styles.header}>
                        <div className={styles.heading}>
                          <h2 className={styles.title}>{block.title}</h2>
                          {editingVisible && block.required ? (
                            <span>
                              {block.hideable ? "Required" : "Always shown"}
                            </span>
                          ) : null}
                        </div>
                        {editingVisible ? (
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
                                    saved
                                      ? current.map((item) =>
                                          item.id === saved.id ? saved : item,
                                        )
                                      : current.filter(
                                          (item) => item.id !== block.id,
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
                      {editingVisible && block.hidden ? (
                        <div className={styles.hiddenNotice}>
                          <span>
                            Hidden from the public page. Everything in it is
                            kept, and it still travels in every download.
                          </span>
                          <button
                            type="button"
                            onClick={() =>
                              void (async () => {
                                try {
                                  const saved = await arrangeAssetBlocks(
                                    assetId,
                                    {
                                      blocks: currentBlocks.map((item) => ({
                                        id: item.id,
                                        hidden:
                                          item.id === block.id
                                            ? false
                                            : item.hidden,
                                        width: item.width,
                                      })),
                                    },
                                  );
                                  setCurrentBlocks(saved);
                                } catch (error) {
                                  setArrangementMessage(
                                    error instanceof Error
                                      ? error.message
                                      : "The section could not be shown. Try again.",
                                  );
                                }
                              })()
                            }
                          >
                            Show it again
                          </button>
                        </div>
                      ) : null}
                      <div className={LAYOUT_CLASS[block.layout]}>
                        {block.elements.map((element) => (
                          <div
                            style={{ gridArea: element.slot }}
                            key={element.id}
                          >
                            <ElementBody
                              element={element}
                              isOwner={editingVisible}
                            />
                          </div>
                        ))}
                      </div>
                    </article>
                  ))}
                </div>
              ))}
            </div>
          )}
        </>
      )}
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
          onRemoved={() =>
            setCurrentBlocks((current) =>
              current.filter((block) => block.id !== editedBlock.id),
            )
          }
          onHide={async () => {
            const saved = await arrangeAssetBlocks(assetId, {
              blocks: currentBlocks.map((block) => ({
                id: block.id,
                hidden: block.id === editedBlock.id ? true : block.hidden,
                width: block.width,
              })),
            });
            setCurrentBlocks(saved);
          }}
          onRemove={() => setRemoving(editedBlock)}
        />
      ) : null}
      {removing ? (
        <RemoveSectionDialog
          block={removing}
          destinations={moveContentDestinations(removing, currentBlocks)}
          pending={blockActionPending}
          error={arrangementMessage}
          onCancel={() => setRemoving(null)}
          onHide={() =>
            runBlockAction(async () => {
              const saved = await arrangeAssetBlocks(assetId, {
                blocks: currentBlocks.map((block) => ({
                  id: block.id,
                  hidden: block.id === removing.id ? true : block.hidden,
                  width: block.width,
                })),
              });
              setCurrentBlocks(saved);
              setRemoving(null);
            })
          }
          onRemove={() =>
            runBlockAction(async () => {
              await removeAssetBlock(assetId, removing.id);
              setCurrentBlocks((current) =>
                current
                  .filter((block) => block.id !== removing.id)
                  .map((block, position) => ({ ...block, position })),
              );
              setRemoving(null);
            })
          }
          onMove={(destinationBlockId) =>
            runBlockAction(async () => {
              const saved = await moveAssetBlockContent(
                assetId,
                removing.id,
                destinationBlockId,
              );
              setCurrentBlocks(saved);
              setRemoving(null);
            })
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
