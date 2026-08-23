"use client";

import { EyeOff, RotateCcw, Trash2, X } from "lucide-react";
import { type FormEvent, useEffect, useRef, useState } from "react";
import {
  type AssetBlock,
  type AssetElement,
  type AssetImage,
  type SaveAssetBlockRequest,
  saveAssetBlock,
} from "@/lib/api/query";
import { LAYOUTS } from "@/lib/page-arrangement";
import { LayoutPicker, WidthPicker } from "./ArrangementPickers";
import styles from "./BlockSheet.module.css";
import { ElementEditor } from "./ElementEditors";
import { ElementOverlay } from "./ElementOverlay";

export function BlockSheet({
  assetId,
  block,
  suggestedWidth,
  images,
  onDismiss,
  onSaved,
  onHide,
  onRemove,
  onImageAdded,
}: {
  assetId: string;
  block: AssetBlock;
  suggestedWidth?: AssetBlock["width"];
  images: AssetImage[];
  onDismiss: () => void;
  onSaved: (block: AssetBlock) => void;
  onHide: () => Promise<void>;
  onRemove: () => void;
  onImageAdded: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [title, setTitle] = useState(block.titleIsDefault ? "" : block.title);
  const [useDefaultTitle, setUseDefaultTitle] = useState(block.titleIsDefault);
  const [elements, setElements] = useState(() =>
    structuredClone(block.elements),
  );
  const [layout, setLayout] = useState(block.layout);
  const [width, setWidth] = useState(block.width);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [arrangementMessage, setArrangementMessage] = useState("");
  const [expanded, setExpanded] = useState<string | null>(null);

  const expandedElement = elements.find((element) => element.id === expanded);

  useEffect(() => {
    dialog.current?.showModal();
  }, []);

  function close() {
    dialog.current?.close();
  }

  function replaceElement(next: AssetElement) {
    setElements((current) =>
      current.map((element) => (element.id === next.id ? next : element)),
    );
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setPending(true);
    setMessage("");
    try {
      const saved = await saveAssetBlock(assetId, block.id, {
        title: useDefaultTitle ? null : title,
        layout,
        width,
        elements: elements.map(toSaveElement),
      });
      onSaved(saved);
      close();
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "The block could not be saved. Try again.",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <dialog
      ref={dialog}
      className={styles.sheet}
      aria-labelledby="block-sheet-title"
      onClose={onDismiss}
    >
      <form className={styles.form} onSubmit={submit}>
        <header className={styles.header}>
          <div>
            <h2 id="block-sheet-title">{block.title}</h2>
          </div>
          <button
            type="button"
            className={styles.close}
            aria-label="Close block editor"
            onClick={close}
          >
            <X size={20} aria-hidden="true" />
          </button>
        </header>

        <div className={styles.scroller}>
          <section className={styles.naming} aria-labelledby="block-name">
            <div className={styles.blockHeading}>
              <h3 id="block-name">Block name</h3>
              {block.required ? (
                <span>{block.hideable ? "Required" : "Always shown"}</span>
              ) : null}
            </div>
            <label htmlFor="block-title">Name on the page</label>
            <input
              id="block-title"
              value={useDefaultTitle ? "" : title}
              placeholder={
                useDefaultTitle ? block.title : "Give this block a name"
              }
              onChange={(event) => {
                setUseDefaultTitle(false);
                setTitle(event.target.value);
              }}
              disabled={pending}
            />
            {useDefaultTitle ? (
              <p className={styles.defaultTitle}>
                <RotateCcw size={15} aria-hidden="true" />
                Using Illarin’s current wording
              </p>
            ) : (
              <button
                type="button"
                className={styles.defaultTitle}
                onClick={() => {
                  setUseDefaultTitle(true);
                  setTitle("");
                }}
                disabled={pending}
              >
                <RotateCcw size={15} aria-hidden="true" />
                Use default name
              </button>
            )}
          </section>

          <section
            className={styles.arrangement}
            aria-labelledby="block-arrangement"
          >
            <div className={styles.arrangementHeading}>
              <div>
                <h3 id="block-arrangement">Page arrangement</h3>
                <p>
                  Layout arranges this block’s content. Width places the block
                  on the desktop page.
                </p>
              </div>
              <div className={styles.arrangementActions}>
                <LayoutPicker
                  layout={layout}
                  width={width}
                  allowedLayouts={block.allowedLayouts}
                  elementLabels={elements.map(
                    (element) => element.label || "Content",
                  )}
                  pending={pending}
                  inline
                  onIssue={setArrangementMessage}
                  onSelect={(choice) => {
                    const slots = LAYOUTS[choice].slots;
                    setLayout(choice);
                    setElements((current) =>
                      current.map((element, index) => ({
                        ...element,
                        slot: slots[index] ?? element.slot,
                      })),
                    );
                  }}
                />
                <WidthPicker
                  width={width}
                  layout={layout}
                  suggestedWidth={suggestedWidth}
                  pending={pending}
                  inline
                  onIssue={setArrangementMessage}
                  onSelect={setWidth}
                />
              </div>
            </div>
            {arrangementMessage ? (
              <p className={styles.arrangementError} role="alert">
                {arrangementMessage}
              </p>
            ) : null}
          </section>

          <div className={styles.elements}>
            {elements.map((element) => (
              <ElementEditor
                key={element.id}
                assetId={assetId}
                element={element}
                images={images}
                pending={pending}
                onChange={replaceElement}
                onImageAdded={onImageAdded}
                onExpand={() => setExpanded(element.id)}
                onRemove={
                  element.pinned
                    ? undefined
                    : () =>
                        setElements((current) =>
                          current.filter((item) => item.id !== element.id),
                        )
                }
              />
            ))}
          </div>

          <section
            className={styles.blockActions}
            aria-labelledby="block-actions"
          >
            <div>
              <h3 id="block-actions">Block actions</h3>
              <p>Hide it from readers, or review what removal would lose.</p>
            </div>
            <div>
              {block.hideable ? (
                <button
                  type="button"
                  disabled={pending}
                  onClick={() => {
                    setPending(true);
                    setMessage("");
                    void onHide()
                      .then(close)
                      .catch((error) =>
                        setMessage(
                          error instanceof Error
                            ? error.message
                            : "The block could not be hidden. Try again.",
                        ),
                      )
                      .finally(() => setPending(false));
                  }}
                >
                  <EyeOff size={16} aria-hidden="true" />
                  Hide block
                </button>
              ) : null}
              {!block.required ? (
                <button
                  type="button"
                  className={styles.removeBlock}
                  disabled={pending}
                  onClick={() => {
                    close();
                    onRemove();
                  }}
                >
                  <Trash2 size={16} aria-hidden="true" />
                  Remove block
                </button>
              ) : null}
            </div>
          </section>

          {message ? (
            <p className={styles.error} role="alert">
              <strong>This block was not saved.</strong> {message}
            </p>
          ) : null}
        </div>

        <footer className={styles.footer}>
          <button type="button" className={styles.cancel} onClick={close}>
            Cancel
          </button>
          <button type="submit" className={styles.save} disabled={pending}>
            {pending ? "Saving…" : "Save block"}
          </button>
        </footer>
      </form>
      {expandedElement ? (
        <ElementOverlay
          assetId={assetId}
          element={expandedElement}
          images={images}
          // The sheet stays open behind it, holding the rest of the block.
          returnLabel={`Return to ${block.title}`}
          pending={false}
          message=""
          onChange={replaceElement}
          onLeave={() => setExpanded(null)}
          onImageAdded={onImageAdded}
        />
      ) : null}
    </dialog>
  );
}

function toSaveElement(
  element: AssetElement,
): SaveAssetBlockRequest["elements"][number] {
  return {
    id: element.id,
    type: element.type,
    role: element.role,
    slot: element.slot,
    display: element.display,
    itemSize: element.itemSize,
    content: element.content,
  };
}
