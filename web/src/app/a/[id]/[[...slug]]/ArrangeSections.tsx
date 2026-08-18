"use client";

import {
  ArrowDown,
  ArrowUp,
  Eye,
  EyeOff,
  GripVertical,
  Trash2,
  X,
} from "lucide-react";
import {
  type CSSProperties,
  type DragEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import {
  type AssetBlock,
  arrangeAssetBlocks,
  removeAssetBlock,
} from "@/lib/api/query";
import {
  contentItemCount,
  packBlockRows,
  WIDTH_LABELS,
} from "@/lib/page-arrangement";
import { WidthPicker } from "./ArrangementPickers";
import styles from "./ArrangeSections.module.css";

export function ArrangeSections({
  assetId,
  blocks,
  onChange,
  onClose,
}: {
  assetId: string;
  blocks: AssetBlock[];
  onChange: (blocks: AssetBlock[]) => void;
  onClose: () => void;
}) {
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [dragged, setDragged] = useState<string | null>(null);
  const [dropAt, setDropAt] = useState<number | null>(null);
  const [removing, setRemoving] = useState<AssetBlock | null>(null);

  async function save(next: AssetBlock[]) {
    if (saving) return;
    setSaving(true);
    setMessage("");
    try {
      const saved = await arrangeAssetBlocks(assetId, {
        blocks: next.map((block) => ({
          id: block.id,
          hidden: block.hidden,
          width: block.width,
        })),
      });
      onChange(saved);
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "The section order could not be saved. Try again.",
      );
    } finally {
      setSaving(false);
    }
  }

  function move(from: number, to: number) {
    if (from === to || saving) return;
    const next = [...blocks];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    void save(next);
  }

  function drop(event: DragEvent, to: number) {
    event.preventDefault();
    const from = blocks.findIndex((block) => block.id === dragged);
    setDragged(null);
    setDropAt(null);
    if (from < 0) return;
    const destination = from < to ? to - 1 : to;
    move(from, Math.min(destination, blocks.length - 1));
  }

  const shapeRows = packBlockRows(blocks, { showHidden: true });

  return (
    <section
      className={styles.arrange}
      aria-labelledby="arrange-sections-title"
    >
      <header className={styles.topline}>
        <div>
          <p className={styles.context}>Page outline</p>
          <h2 id="arrange-sections-title">Arrange sections</h2>
          <p>
            Move, resize, hide or remove sections without scrolling the page.
          </p>
        </div>
        <button type="button" className={styles.close} onClick={onClose}>
          <X size={17} aria-hidden="true" />
          Done arranging
        </button>
      </header>

      <div className={styles.shape} role="img" aria-label="Page shape">
        {shapeRows.map((row) => (
          <div
            className={styles.shapeRow}
            key={row.map(({ block }) => block.id).join(":")}
          >
            {row.map(({ block, columns }) => (
              <div
                className={styles.shapeBlock}
                data-hidden={block.hidden || undefined}
                key={block.id}
                style={{ "--shape-columns": columns } as CSSProperties}
                title={`${block.title}, ${WIDTH_LABELS[block.width]}`}
              >
                <span>{block.title}</span>
              </div>
            ))}
          </div>
        ))}
      </div>

      <p className={styles.mobileWidthNote}>
        Widths describe the desktop page. Every section stays full width here.
      </p>
      {message ? (
        <p className={styles.message} role="alert">
          {message}
        </p>
      ) : null}

      <ol className={styles.outline} aria-label="Sections in page order">
        {blocks.map((block, index) => (
          <li
            className={styles.outlineRow}
            data-dragging={dragged === block.id || undefined}
            data-drop-before={dropAt === index || undefined}
            key={block.id}
            onDragOver={(event) => {
              event.preventDefault();
              setDropAt(index);
            }}
            onDrop={(event) => drop(event, index)}
          >
            <button
              type="button"
              className={styles.handle}
              draggable
              aria-label={`Drag ${block.title} to reorder`}
              onDragStart={(event) => {
                setDragged(block.id);
                event.dataTransfer.effectAllowed = "move";
                event.dataTransfer.setData("text/plain", block.id);
              }}
              onDragEnd={() => {
                setDragged(null);
                setDropAt(null);
              }}
            >
              <GripVertical size={19} aria-hidden="true" />
            </button>

            <div className={styles.rowIdentity}>
              <div className={styles.rowTitle}>
                <strong>{block.title}</strong>
                {block.required ? (
                  <span>{block.hideable ? "Required" : "Always shown"}</span>
                ) : null}
                {block.hidden ? (
                  <span className={styles.hiddenChip}>Hidden</span>
                ) : null}
              </div>
              <p>
                {block.elements.length}{" "}
                {block.elements.length === 1 ? "element" : "elements"}
              </p>
            </div>

            <div className={styles.mobileMove}>
              <button
                type="button"
                aria-label={`Move ${block.title} up`}
                disabled={saving || index === 0}
                onClick={() => move(index, index - 1)}
              >
                <ArrowUp size={17} aria-hidden="true" />
              </button>
              <button
                type="button"
                aria-label={`Move ${block.title} down`}
                disabled={saving || index === blocks.length - 1}
                onClick={() => move(index, index + 1)}
              >
                <ArrowDown size={17} aria-hidden="true" />
              </button>
              <label>
                <span className={styles.srOnly}>Move {block.title} to</span>
                <select
                  value=""
                  disabled={saving || blocks.length < 2}
                  onChange={(event) => {
                    const destination = Number(event.currentTarget.value);
                    if (Number.isInteger(destination)) move(index, destination);
                  }}
                >
                  <option value="">Move to…</option>
                  {moveDestinations(blocks, index).map((destination) => (
                    <option value={destination.index} key={destination.index}>
                      {destination.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <div className={styles.rowActions}>
              <div className={styles.outlineWidth}>
                <WidthPicker
                  width={block.width}
                  layout={block.layout}
                  pending={saving}
                  onIssue={setMessage}
                  onSelect={(width) =>
                    void save(
                      blocks.map((item) =>
                        item.id === block.id ? { ...item, width } : item,
                      ),
                    )
                  }
                />
              </div>
              {block.hideable ? (
                <button
                  type="button"
                  disabled={saving}
                  onClick={() =>
                    void save(
                      blocks.map((item) =>
                        item.id === block.id
                          ? { ...item, hidden: !item.hidden }
                          : item,
                      ),
                    )
                  }
                >
                  {block.hidden ? (
                    <Eye size={17} aria-hidden="true" />
                  ) : (
                    <EyeOff size={17} aria-hidden="true" />
                  )}
                  {block.hidden ? "Show" : "Hide"}
                </button>
              ) : null}
              {!block.required ? (
                <button
                  type="button"
                  className={styles.remove}
                  disabled={saving}
                  onClick={() => setRemoving(block)}
                >
                  <Trash2 size={17} aria-hidden="true" />
                  Remove
                </button>
              ) : null}
            </div>
          </li>
        ))}
        <li
          className={styles.lastDrop}
          data-active={dropAt === blocks.length || undefined}
          onDragOver={(event) => {
            event.preventDefault();
            setDropAt(blocks.length);
          }}
          onDrop={(event) => drop(event, blocks.length)}
        />
      </ol>

      {removing ? (
        <RemoveSectionDialog
          block={removing}
          pending={saving}
          onCancel={() => setRemoving(null)}
          onHide={async () => {
            setRemoving(null);
            await save(
              blocks.map((item) =>
                item.id === removing.id ? { ...item, hidden: true } : item,
              ),
            );
          }}
          onRemove={async () => {
            if (saving) return;
            setSaving(true);
            setMessage("");
            try {
              await removeAssetBlock(assetId, removing.id);
              onChange(
                blocks
                  .filter((block) => block.id !== removing.id)
                  .map((block, position) => ({ ...block, position })),
              );
              setRemoving(null);
            } catch (error) {
              setMessage(
                error instanceof Error
                  ? error.message
                  : "The section could not be removed. Try again.",
              );
            } finally {
              setSaving(false);
            }
          }}
        />
      ) : null}
    </section>
  );
}

function moveDestinations(blocks: AssetBlock[], current: number) {
  const remaining = blocks.filter((_, index) => index !== current);
  return Array.from({ length: blocks.length }, (_, index) => index)
    .filter((index) => index !== current)
    .map((index) => ({
      index,
      label:
        index === remaining.length
          ? `After “${remaining.at(-1)?.title}”`
          : `Before “${remaining[index].title}”`,
    }));
}

function RemoveSectionDialog({
  block,
  pending,
  onCancel,
  onHide,
  onRemove,
}: {
  block: AssetBlock;
  pending: boolean;
  onCancel: () => void;
  onHide: () => void;
  onRemove: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => dialog.current?.showModal(), []);
  const losses = block.elements
    .map((element) => ({ element, count: contentItemCount(element) }))
    .filter(({ count }) => count > 0);
  const movable = block.elements.filter((element) => !element.pinned);

  return (
    <dialog
      ref={dialog}
      className={styles.dialog}
      onCancel={(event) => {
        event.preventDefault();
        onCancel();
      }}
    >
      <div className={styles.dialogBody}>
        <p className={styles.context}>Remove section</p>
        <h2>Remove “{block.title}”?</h2>
        {losses.length > 0 ? (
          <>
            <p>This removes the following content:</p>
            <ul className={styles.losses}>
              {losses.map(({ element, count }) => (
                <li key={element.id}>
                  <strong>{element.label}</strong>
                  <span>
                    {count} {count === 1 ? "item" : "items"}, and it is what a
                    download reads for {element.label}
                  </span>
                </li>
              ))}
            </ul>
            <p className={styles.noCopy}>
              There is nowhere else this content is kept. The section is where
              it lives.
            </p>
          </>
        ) : (
          <p>This section is empty, so nothing is lost.</p>
        )}

        <section className={styles.keep} aria-labelledby="keep-section-heading">
          <h3 id="keep-section-heading">Or keep it</h3>
          {block.hideable ? (
            <button type="button" disabled={pending} onClick={onHide}>
              <EyeOff size={18} aria-hidden="true" />
              <span>
                <strong>Hide the section</strong>
                Everything stays in downloads and leaves the public page.
              </span>
            </button>
          ) : null}
          {movable.length > 0 ? (
            <div className={styles.keepChoice}>
              <GripVertical size={18} aria-hidden="true" />
              <span>
                <strong>Move the content first</strong>
                Move unpinned elements into another section, then return here.
              </span>
            </div>
          ) : null}
        </section>
      </div>
      <footer className={styles.dialogFooter}>
        <button type="button" onClick={onCancel} disabled={pending}>
          Cancel
        </button>
        <button
          type="button"
          className={styles.confirmRemove}
          onClick={onRemove}
          disabled={pending}
        >
          <Trash2 size={17} aria-hidden="true" />
          {pending ? "Removing…" : "Remove and delete"}
        </button>
      </footer>
    </dialog>
  );
}
