"use client";

import { PencilLine } from "lucide-react";
import { useRouter } from "next/navigation";
import {
  type CSSProperties,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  type AddableBlock,
  type AssetBlock,
  type AssetElement,
  type AssetImage,
  addAssetBlock,
  arrangeAssetBlocks,
  type BrowseKind,
  type ElementType,
  moveAssetBlockContent,
  removeAssetBlock,
  type SaveAssetBlockRequest,
  saveAssetBlock,
} from "@/lib/api/query";
import { blockCounts } from "@/lib/asset-block-heading";
import {
  assetHoldsNothing,
  coreBlockTitles,
  rendersOnThePage,
  splitAssetPageContent,
} from "@/lib/asset-page-content";
import {
  BLOCK_GRID_GAP_PX,
  type BlockWidth,
  elementTracks,
  ornamentPlacement,
  packBlockRows,
  pageFullness,
  suggestedBlockWidth,
  suggestionCandidateWidths,
} from "@/lib/page-arrangement";
import { useMeasuredWidth } from "@/lib/use-measured-width";
import { AddBlockTray } from "./AddBlockTray";
import {
  ArrangeBlocks,
  moveContentDestinations,
  RemoveBlockDialog,
} from "./ArrangeBlocks";
import { WidthPicker } from "./ArrangementPickers";
import styles from "./AssetBlocks.module.css";
import { BlockSheet } from "./BlockSheet";
import { ContentsBar } from "./ContentsBar";
import type { CreatorMenuProps } from "./CreatorMenu";
import { ElementBody } from "./ElementBody";
import { ElementOverlay } from "./ElementOverlay";
import { ElementReader } from "./ElementReader";
import {
  type ArtPlacement,
  EmptyPage,
  EmptyPageInvitation,
  QuietPageArt,
} from "./QuietPage";

function measureCandidateHeights(
  source: HTMLElement,
  layout: AssetBlock["layout"],
  availableWidth: number,
): Partial<Record<BlockWidth, number>> {
  const heights: Partial<Record<BlockWidth, number>> = {};

  for (const candidate of suggestionCandidateWidths(layout, availableWidth)) {
    const clone = source.cloneNode(true) as HTMLElement;
    clone.removeAttribute("id");
    clone.setAttribute("aria-hidden", "true");
    clone.inert = true;
    Object.assign(clone.style, {
      position: "fixed",
      inset: "0 auto auto -100000px",
      width: `${candidate.renderedWidth}px`,
      maxWidth: "none",
      gridColumn: "auto",
      visibility: "hidden",
      pointerEvents: "none",
    });
    for (const identified of clone.querySelectorAll("[id]")) {
      identified.removeAttribute("id");
    }
    for (const ignored of clone.querySelectorAll(
      "[data-empty], [data-measurement-ignore]",
    )) {
      ignored.remove();
    }

    document.body.append(clone);
    for (const excerpt of clone.querySelectorAll<HTMLElement>(
      "[data-line-excerpt]",
    )) {
      const isCut = excerpt.scrollHeight - excerpt.clientHeight > 1;
      const sibling = excerpt.nextElementSibling;
      const readMore =
        sibling instanceof HTMLElement && sibling.hasAttribute("data-read-more")
          ? sibling
          : null;
      if (!isCut) {
        readMore?.remove();
      } else if (!readMore) {
        const readMoreSpace = document.createElement("span");
        readMoreSpace.style.display = "block";
        readMoreSpace.style.height = "20px";
        excerpt.after(readMoreSpace);
      }
    }

    const content = clone.querySelector<HTMLElement>("[data-block-content]");
    const height = content?.getBoundingClientRect().height ?? 0;
    if (height > 0) heights[candidate.width] = height;
    clone.remove();
  }

  return heights;
}

function returnToBlock(blockId: string) {
  const anchor = `block-${blockId}`;
  document.getElementById(anchor)?.scrollIntoView({ block: "start" });
  window.location.hash = anchor;
}

/** The asset's content. An owner also sees the blocks they have yet to fill. */
export function AssetBlocks({
  assetId,
  kind,
  blocks,
  images,
  addableBlocks,
  isOwner,
  creatorMenu,
}: {
  assetId: string;
  kind: BrowseKind;
  blocks: AssetBlock[];
  images: AssetImage[];
  addableBlocks: AddableBlock[];
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
  const [suggestedWidths, setSuggestedWidths] = useState<
    Record<string, BlockWidth>
  >({});
  const rowsNode = useRef<HTMLDivElement | null>(null);

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
  const { publicBlocks, modelContent: disclosedModelContent } = useMemo(
    () => splitAssetPageContent(currentBlocks),
    [currentBlocks],
  );
  const modelContent = editingVisible ? [] : disclosedModelContent;
  const [rowsRef, availableWidth] = useMeasuredWidth<HTMLDivElement>();
  const setRowsRef = useCallback(
    (node: HTMLDivElement | null) => {
      rowsNode.current = node;
      rowsRef(node);
    },
    [rowsRef],
  );
  const packable: Array<AssetBlock & { empty?: boolean }> = editingVisible
    ? currentBlocks
    : publicBlocks.filter(rendersOnThePage);
  const rows = packBlockRows(packable, {
    availableWidth,
  });
  const fullness = pageFullness(rows);
  /** An owner filling in an empty page is invited once, not block by block. */
  const invited = editingVisible && assetHoldsNothing(currentBlocks);
  const ornament = invited
    ? null
    : ornamentPlacement(rows, holdsCreatorPictures);
  const ornamentAtFoot =
    !invited &&
    rows.length > 0 &&
    !ornament &&
    !rows.some(holdsCreatorPictures);
  const contentsBlocks = useMemo(
    () =>
      editingVisible ? currentBlocks : publicBlocks.filter(rendersOnThePage),
    [currentBlocks, editingVisible, publicBlocks],
  );

  const editedBlock = currentBlocks.find((block) => block.id === editing);
  const expandedBlock = expanding
    ? currentBlocks.find((block) => block.id === expanding.blockId)
    : undefined;

  const measureSuggestedWidths = useCallback(() => {
    if (!rowsNode.current || availableWidth === undefined) return;
    const next: Record<string, BlockWidth> = {};
    const blocksById = new Map(currentBlocks.map((block) => [block.id, block]));
    for (const node of rowsNode.current.querySelectorAll<HTMLElement>(
      "[data-block-id]",
    )) {
      const blockId = node.dataset.blockId;
      const content = node.querySelector<HTMLElement>("[data-block-content]");
      const block = blockId ? blocksById.get(blockId) : undefined;
      if (!blockId || !block || block.isEmpty || !content) continue;
      const suggestion = suggestedBlockWidth({
        width: block.width,
        layout: block.layout,
        availableWidth,
        renderedHeights: measureCandidateHeights(
          node,
          block.layout,
          availableWidth,
        ),
      });
      if (suggestion) next[blockId] = suggestion;
    }
    setSuggestedWidths((current) =>
      sameWidthSuggestions(current, next) ? current : next,
    );
  }, [availableWidth, currentBlocks]);

  useEffect(() => {
    if (
      !editingVisible ||
      editing ||
      expanding ||
      arranging ||
      !rowsNode.current
    )
      return;
    let frame = window.requestAnimationFrame(measureSuggestedWidths);
    const observer = new ResizeObserver(() => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(measureSuggestedWidths);
    });
    observer.observe(rowsNode.current);
    for (const content of rowsNode.current.querySelectorAll<HTMLElement>(
      "[data-block-content]",
    )) {
      observer.observe(content);
    }
    return () => {
      window.cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [arranging, editingVisible, editing, expanding, measureSuggestedWidths]);

  /** A save that fails keeps the overlay open rather than closing over a loss. */
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
        current.map((item) => (item.id === saved.id ? saved : item)),
      );
      setExpanding(null);
      returnToBlock(saved.id);
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

  function addBlock(definition: string, elementType: ElementType) {
    void runBlockAction(async () => {
      const added = await addAssetBlock(assetId, definition, elementType);
      setCurrentBlocks((current) => [...current, added]);
      setArranging(false);
      setAdding(false);
      setAdded(added.id);
    });
  }

  async function runBlockAction(action: () => Promise<void>) {
    if (blockActionPending) return;
    setBlockActionPending(true);
    setArrangementMessage("");
    try {
      await action();
      router.refresh();
    } catch (error) {
      setArrangementMessage(
        error instanceof Error
          ? error.message
          : "The block could not be changed. Try again.",
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
        canAdd={addableBlocks.length > 0}
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
        <ArrangeBlocks
          assetId={assetId}
          blocks={currentBlocks}
          suggestedWidths={suggestedWidths}
          onChange={setCurrentBlocks}
          onClose={() => setArranging(false)}
        />
      ) : (
        <>
          {editingVisible ? (
            <p className={styles.narrowNote}>
              Block widths arrange the desktop page. On this screen every block
              fills the width, and no content is lost.
            </p>
          ) : null}
          {arrangementMessage ? (
            <p className={styles.arrangementMessage} role="alert">
              {arrangementMessage}
            </p>
          ) : null}
          {invited ? (
            <EmptyPageInvitation
              kind={kind}
              coreBlocks={coreBlockTitles(currentBlocks)}
              canAdd={addableBlocks.length > 0}
            />
          ) : fullness === "empty" ? (
            <EmptyPage kind={kind} />
          ) : null}
          {rows.length === 0 ? null : (
            <div
              className={styles.rows}
              ref={setRowsRef}
              style={
                {
                  "--block-grid-gap": `${BLOCK_GRID_GAP_PX}px`,
                } as CSSProperties
              }
            >
              {rows.map((row, rowIndex) => (
                <div
                  className={styles.row}
                  key={row.map((item) => item.block.id).join(":")}
                >
                  {row.map(({ block, columns, startColumn }) => (
                    <article
                      id={`block-${block.id}`}
                      key={block.id}
                      className={styles.block}
                      data-block-id={block.id}
                      data-hidden={
                        editingVisible && block.hidden ? true : undefined
                      }
                      style={
                        {
                          "--block-columns": columns,
                          "--block-start": startColumn,
                        } as CSSProperties
                      }
                    >
                      <header className={styles.header}>
                        <div className={styles.heading}>
                          <h2 className={styles.title}>{block.title}</h2>
                          {editingVisible && block.required ? (
                            <span>
                              {block.hideable ? "Required" : "Always shown"}
                            </span>
                          ) : null}
                          <BlockCounts elements={block.elements} />
                        </div>
                        {editingVisible ? (
                          <div className={styles.controls}>
                            <WidthPicker
                              width={block.width}
                              layout={block.layout}
                              suggestedWidth={suggestedWidths[block.id]}
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
                              <span>Edit block</span>
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
                                      : "The block could not be shown. Try again.",
                                  );
                                }
                              })()
                            }
                          >
                            Show it again
                          </button>
                        </div>
                      ) : null}
                      <div
                        className={styles.elements}
                        data-block-content
                        style={
                          {
                            "--element-tracks": elementTracks(
                              block.layout,
                              block.elements.length,
                            ),
                          } as CSSProperties
                        }
                      >
                        {block.elements.map((element) => (
                          <div
                            key={element.id}
                            data-empty={element.isEmpty ? true : undefined}
                          >
                            <ElementBody
                              element={element}
                              isOwner={editingVisible}
                              images={images}
                              blockTitle={block.title}
                              blockElements={block.elements.length}
                              markEmpty={!invited}
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
                  {ornament?.row === rowIndex ? (
                    <Ornament
                      kind={kind}
                      barren={fullness === "barren"}
                      placement="inRow"
                      style={
                        {
                          "--block-columns": ornament.columns,
                          "--block-start": ornament.startColumn,
                        } as CSSProperties
                      }
                    />
                  ) : null}
                </div>
              ))}
              {ornamentAtFoot ? (
                <Ornament
                  kind={kind}
                  barren={fullness === "barren"}
                  placement="atFoot"
                />
              ) : null}
            </div>
          )}
          {modelContent.length > 0 ? (
            <details className={styles.modelContent}>
              <summary>
                <span className={styles.modelContentTitle}>
                  Model-facing content
                </span>
                <span className={styles.modelContentSummary}>
                  System prompt and post-history instructions
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
            <AddBlockTray
              addable={addableBlocks}
              blocks={currentBlocks}
              pending={blockActionPending}
              onAdd={addBlock}
              onClose={() => setAdding(false)}
            />
          ) : null}
        </>
      )}
      {editedBlock ? (
        <BlockSheet
          assetId={assetId}
          block={editedBlock}
          suggestedWidth={suggestedWidths[editedBlock.id]}
          images={images}
          onDismiss={() => setEditing(null)}
          onImageAdded={() => router.refresh()}
          onSaved={(saved) => {
            setCurrentBlocks((current) =>
              current.map((block) => (block.id === saved.id ? saved : block)),
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
        <RemoveBlockDialog
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

/**
 * The artwork that takes what a row leaves. A page with plenty on it gets the
 * shared wash. A page with one row gets the kind's own piece, because there
 * the artwork is the composition rather than a hint of one.
 */
function Ornament({
  kind,
  barren,
  placement,
  style,
}: {
  kind: BrowseKind;
  barren: boolean;
  placement: Extract<ArtPlacement, "inRow" | "atFoot">;
  style?: CSSProperties;
}) {
  if (barren) {
    return <QuietPageArt kind={kind} placement={placement} style={style} />;
  }
  return (
    <div
      aria-hidden="true"
      data-measurement-ignore
      className={`${styles.ornament} ${styles[placement]}`}
      style={style}
    />
  );
}

function BlockCounts({ elements }: { elements: AssetElement[] }) {
  const counts = blockCounts(elements);
  if (!counts) return null;
  return <p className={styles.counts}>{counts}</p>;
}

function holdsCreatorPictures(row: readonly { block: AssetBlock }[]): boolean {
  return row.some(({ block }) =>
    block.elements.some(
      (element) => element.type === "image_set" && !element.isEmpty,
    ),
  );
}

function sameWidthSuggestions(
  current: Record<string, BlockWidth>,
  next: Record<string, BlockWidth>,
) {
  const currentIds = Object.keys(current);
  const nextIds = Object.keys(next);
  return (
    currentIds.length === nextIds.length &&
    currentIds.every((id) => current[id] === next[id])
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
