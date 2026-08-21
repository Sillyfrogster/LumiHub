"use client";

import { PencilLine } from "lucide-react";
import { useRouter } from "next/navigation";
import { type CSSProperties, useEffect, useMemo, useState } from "react";
import {
  type AddableSection,
  type AssetBlock,
  type AssetElement,
  type AssetImage,
  addAssetBlock,
  arrangeAssetBlocks,
  type ElementType,
  moveAssetBlockContent,
  removeAssetBlock,
  type SaveAssetBlockRequest,
  saveAssetBlock,
} from "@/lib/api/query";
import { packBlockRows } from "@/lib/page-arrangement";
import { AddSectionTray } from "./AddSectionTray";
import { WidthPicker } from "./ArrangementPickers";
import {
  ArrangeSections,
  moveContentDestinations,
  RemoveSectionDialog,
} from "./ArrangeSections";
import styles from "./AssetBlocks.module.css";
import { BlockSheet } from "./BlockSheet";
import { ContentsBar } from "./ContentsBar";
import type { CreatorMenuProps } from "./CreatorMenu";
import { ElementBody } from "./ElementBody";
import { ElementOverlay } from "./ElementOverlay";
import { ElementReader } from "./ElementReader";

const MODEL_FACING_ROLES = new Set([
  "description",
  "personality",
  "scenario",
  "system_prompt",
  "post_history_instructions",
]);

function returnToBlock(blockId: string) {
  const anchor = `block-${blockId}`;
  document.getElementById(anchor)?.scrollIntoView({ block: "start" });
  window.location.hash = anchor;
}

/** One row arrangement per layout preset the catalog can choose. */
const LAYOUT_CLASS: Record<AssetBlock["layout"], string> = {
  single: styles.single,
  duo: styles.duo,
  "main-aside": styles.mainAside,
  trio: styles.trio,
  "stack-2": styles.stack2,
  "stack-3": styles.stack3,
};

function isModelFacingElement(element: AssetElement) {
  return MODEL_FACING_ROLES.has(element.role ?? "");
}

/**
 * The asset's content, in page order. A reader is shown the blocks that carry
 * something. The owner is shown every block the kind asks for, because an
 * empty required block is what says what is still theirs to write.
 */
export function AssetBlocks({
  assetId,
  blocks,
  images,
  addableSections,
  isOwner,
  creatorMenu,
}: {
  assetId: string;
  blocks: AssetBlock[];
  images: AssetImage[];
  addableSections: AddableSection[];
  isOwner: boolean;
  creatorMenu: CreatorMenuProps;
}) {
  const router = useRouter();
  const [currentBlocks, setCurrentBlocks] = useState(blocks);
  const [editing, setEditing] = useState<string | null>(null);
  const [savingWidth, setSavingWidth] = useState<string | null>(null);
  const [arrangementMessage, setArrangementMessage] = useState("");
  const [arranging, setArranging] = useState(false);
  const [adding, setAdding] = useState(false);
  const [readerView, setReaderView] = useState(false);
  const [removing, setRemoving] = useState<AssetBlock | null>(null);
  const [blockActionPending, setBlockActionPending] = useState(false);
  const [added, setAdded] = useState<string | null>(null);
  const [expanding, setExpanding] = useState<{
    blockId: string;
    element: AssetElement;
  } | null>(null);
  const [reading, setReading] = useState<{
    blockId: string;
    element: AssetElement;
  } | null>(null);
  const [expandPending, setExpandPending] = useState(false);
  const [expandMessage, setExpandMessage] = useState("");

  useEffect(() => setCurrentBlocks(blocks), [blocks]);

  useEffect(() => {
    if (!added) return;
    returnToBlock(added);
    setAdded(null);
  }, [added]);

  useEffect(() => {
    if (!adding) return;
    window.requestAnimationFrame(() => {
      document
        .getElementById("add-block-tray")
        ?.scrollIntoView({ block: "start" });
    });
  }, [adding]);

  const editingVisible = isOwner && !readerView;
  const readerBlocks = useMemo(
    () =>
      currentBlocks.flatMap((block) => {
        const elements = block.elements.filter(
          (element) => !element.isEmpty && !isModelFacingElement(element),
        );
        return elements.length > 0 ? [{ ...block, elements }] : [];
      }),
    [currentBlocks],
  );
  const modelContent = editingVisible
    ? []
    : currentBlocks.flatMap((block) =>
        block.hidden
          ? []
          : block.elements
              .filter(
                (element) => !element.isEmpty && isModelFacingElement(element),
              )
              .map((element) => ({ block, element })),
      );
  const packable = editingVisible ? currentBlocks : readerBlocks;
  const rows = packBlockRows(packable, { showHidden: editingVisible });
  const contentsBlocks = useMemo(
    () =>
      editingVisible
        ? currentBlocks
        : readerBlocks.filter((block) => !block.hidden),
    [currentBlocks, editingVisible, readerBlocks],
  );

  const editedBlock = currentBlocks.find((block) => block.id === editing);
  const expandedBlock = expanding
    ? currentBlocks.find((block) => block.id === expanding.blockId)
    : undefined;

  /**
   * Leaving the overlay writes the element through and puts the creator back
   * on the section it came from. A save that fails keeps the overlay open
   * holding the editing, rather than closing over a loss.
   */
  async function leaveOverlay() {
    if (!expanding || !expandedBlock || expandPending) return;
    setExpandPending(true);
    setExpandMessage("");
    try {
      const saved = await saveAssetBlock(
        assetId,
        expandedBlock.id,
        blockSaveRequest(expandedBlock, {
          elements: expandedBlock.elements.map((element) =>
            element.id === expanding.element.id ? expanding.element : element,
          ),
        }),
      );
      setCurrentBlocks((current) =>
        saved
          ? current.map((item) => (item.id === saved.id ? saved : item))
          : current.filter((item) => item.id !== expandedBlock.id),
      );
      setExpanding(null);
      if (saved) returnToBlock(saved.id);
    } catch (error) {
      setExpandMessage(
        error instanceof Error
          ? error.message
          : "The content could not be saved. Try again.",
      );
    } finally {
      setExpandPending(false);
    }
  }

  function dismissReader() {
    const elementId = reading?.element.id;
    setReading(null);
    if (!elementId) return;
    window.requestAnimationFrame(() => {
      document.getElementById(`read-${elementId}`)?.focus({
        preventScroll: true,
      });
    });
  }

  function addSection(definition: string, elementType: ElementType) {
    void runBlockAction(async () => {
      const section = await addAssetBlock(assetId, definition, elementType);
      setCurrentBlocks((current) => [...current, section]);
      setArranging(false);
      setAdding(false);
      setAdded(section.id);
    });
  }

  async function runBlockAction(action: () => Promise<void>) {
    if (blockActionPending) return;
    setBlockActionPending(true);
    setArrangementMessage("");
    try {
      await action();
      // The download panel is rendered from the asset's projection, which the
      // save just rewrote, so the page is read again.
      router.refresh();
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
      <ContentsBar
        blocks={contentsBlocks}
        isOwner={isOwner}
        arranging={arranging}
        adding={adding}
        readerView={readerView}
        canAdd={addableSections.length > 0}
        creatorMenu={creatorMenu}
        onToggleArrange={() => {
          setEditing(null);
          setAdding(false);
          setArranging((current) => !current);
        }}
        onToggleAdd={() => {
          setEditing(null);
          setArranging(false);
          setAdding((current) => !current);
        }}
        onReaderView={() => {
          setEditing(null);
          setAdding(false);
          setArranging(false);
          setReaderView(true);
        }}
        onReturnToEditing={() => setReaderView(false)}
      />

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
                                    blockSaveRequest(block, { width }),
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
                              images={images}
                              blockTitle={block.title}
                              onExpand={() =>
                                setExpanding({
                                  blockId: block.id,
                                  element: structuredClone(element),
                                })
                              }
                              onReadMore={() =>
                                setReading({
                                  blockId: block.id,
                                  element,
                                })
                              }
                            />
                            {reading?.blockId === block.id &&
                            reading.element.id === element.id ? (
                              <ElementReader
                                element={reading.element}
                                images={images}
                                onDismiss={dismissReader}
                              />
                            ) : null}
                          </div>
                        ))}
                      </div>
                    </article>
                  ))}
                </div>
              ))}
            </div>
          )}
          {modelContent.length > 0 ? (
            <details className={styles.modelContent}>
              <summary>
                <span className={styles.modelContentTitle}>
                  Model-facing content
                </span>
                <span className={styles.modelContentSummary}>
                  Description, personality, scenario and instructions
                </span>
              </summary>
              <div className={styles.modelContentBody}>
                {modelContent.map(({ block, element }) => (
                  <div key={element.id}>
                    <ElementBody
                      element={element}
                      isOwner={false}
                      images={images}
                      onReadMore={() =>
                        setReading({
                          blockId: block.id,
                          element,
                        })
                      }
                    />
                    {reading?.blockId === block.id &&
                    reading.element.id === element.id ? (
                      <ElementReader
                        element={reading.element}
                        images={images}
                        onDismiss={dismissReader}
                      />
                    ) : null}
                  </div>
                ))}
              </div>
            </details>
          ) : null}
          {editingVisible && adding ? (
            <AddSectionTray
              sections={addableSections}
              blocks={currentBlocks}
              pending={blockActionPending}
              onAdd={addSection}
              onClose={() => setAdding(false)}
            />
          ) : null}
        </>
      )}
      {editedBlock ? (
        <BlockSheet
          assetId={assetId}
          block={editedBlock}
          images={images}
          onDismiss={() => setEditing(null)}
          onImageAdded={() => router.refresh()}
          onSaved={(saved) => {
            setCurrentBlocks((current) =>
              current.map((block) => (block.id === saved.id ? saved : block)),
            );
            router.refresh();
          }}
          onRemoved={() => {
            setCurrentBlocks((current) =>
              current.filter((block) => block.id !== editedBlock.id),
            );
            router.refresh();
          }}
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
      {expanding && expandedBlock ? (
        <ElementOverlay
          assetId={assetId}
          element={expanding.element}
          images={images}
          returnLabel={`Return to ${expandedBlock.title}`}
          pending={expandPending}
          message={expandMessage}
          onChange={(element) =>
            setExpanding((current) =>
              current ? { ...current, element } : current,
            )
          }
          onLeave={() => void leaveOverlay()}
          onImageAdded={() => router.refresh()}
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
  changes: { width?: AssetBlock["width"]; elements?: AssetElement[] },
): SaveAssetBlockRequest {
  return {
    title: block.titleIsDefault ? null : block.title,
    layout: block.layout,
    width: changes.width ?? block.width,
    elements: (changes.elements ?? block.elements).map((element) => ({
      id: element.id,
      type: element.type,
      role: element.role,
      slot: element.slot,
      display: element.display,
      itemSize: element.itemSize,
      content: element.content,
    })),
  };
}
