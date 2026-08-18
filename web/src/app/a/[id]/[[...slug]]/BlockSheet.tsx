"use client";

import { Plus, RotateCcw, Trash2, X } from "lucide-react";
import { type FormEvent, useEffect, useRef, useState } from "react";
import {
  type AssetBlock,
  type AssetElement,
  type SaveAssetBlockRequest,
  saveAssetBlock,
} from "@/lib/api/query";
import styles from "./BlockSheet.module.css";

type TextItem = { name?: string; text: string };
type DialogueTurn = { speaker: string; text: string };

export function BlockSheet({
  assetId,
  block,
  onDismiss,
  onSaved,
}: {
  assetId: string;
  block: AssetBlock;
  onDismiss: () => void;
  onSaved: (block: AssetBlock) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [title, setTitle] = useState(block.titleIsDefault ? "" : block.title);
  const [useDefaultTitle, setUseDefaultTitle] = useState(block.titleIsDefault);
  const [elements, setElements] = useState(() =>
    structuredClone(block.elements),
  );
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

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
        elements: elements.map(toSaveElement),
      });
      onSaved(saved);
      close();
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "The section could not be saved. Try again.",
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
            aria-label="Close section editor"
            onClick={close}
          >
            <X size={20} aria-hidden="true" />
          </button>
        </header>

        <div className={styles.scroller}>
          <section className={styles.naming} aria-labelledby="section-name">
            <div className={styles.sectionHeading}>
              <h3 id="section-name">Section name</h3>
              {block.required ? (
                <span>{block.hideable ? "Required" : "Always shown"}</span>
              ) : null}
            </div>
            <label htmlFor="block-title">Name on the page</label>
            <input
              id="block-title"
              value={useDefaultTitle ? "" : title}
              placeholder={
                useDefaultTitle ? block.title : "Give this section a name"
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

          <div className={styles.elements}>
            {elements.map((element) => (
              <ElementEditor
                key={element.id}
                element={element}
                pending={pending}
                onChange={replaceElement}
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

          {message ? (
            <p className={styles.error} role="alert">
              <strong>This section was not saved.</strong> {message}
            </p>
          ) : null}
        </div>

        <footer className={styles.footer}>
          <button type="button" className={styles.cancel} onClick={close}>
            Cancel
          </button>
          <button type="submit" className={styles.save} disabled={pending}>
            {pending ? "Saving…" : "Save section"}
          </button>
        </footer>
      </form>
    </dialog>
  );
}

function ElementEditor({
  element,
  pending,
  onChange,
  onRemove,
}: {
  element: AssetElement;
  pending: boolean;
  onChange: (element: AssetElement) => void;
  onRemove?: () => void;
}) {
  return (
    <section
      className={styles.element}
      aria-labelledby={`element-${element.id}`}
    >
      <div className={styles.elementHeading}>
        <div>
          <h3 id={`element-${element.id}`}>{element.label || "Content"}</h3>
          <p>{elementHint(element.type)}</p>
        </div>
        {onRemove ? (
          <button type="button" onClick={onRemove} disabled={pending}>
            <Trash2 size={15} aria-hidden="true" />
            Remove
          </button>
        ) : null}
      </div>
      <ElementFields element={element} pending={pending} onChange={onChange} />
    </section>
  );
}

function ElementFields({
  element,
  pending,
  onChange,
}: {
  element: AssetElement;
  pending: boolean;
  onChange: (element: AssetElement) => void;
}) {
  if (element.type === "prose" && "text" in element.content) {
    return (
      <label className={styles.prose}>
        <span>{element.label || "Text"}</span>
        <textarea
          rows={11}
          value={element.content.text}
          onChange={(event) =>
            onChange({
              ...element,
              content: { text: event.target.value },
              isEmpty: event.target.value.trim() === "",
            })
          }
          disabled={pending}
        />
      </label>
    );
  }

  if (element.type === "text_set" && "texts" in element.content) {
    return (
      <ListEditor
        items={element.content.texts}
        pending={pending}
        noun="greeting"
        onChange={(texts) =>
          onChange({
            ...element,
            content: { texts },
            isEmpty: texts.every((item) => item.text.trim() === ""),
          })
        }
      />
    );
  }

  if (element.type === "dialogue_sample" && "turns" in element.content) {
    return (
      <DialogueEditor
        turns={element.content.turns}
        pending={pending}
        onChange={(turns) =>
          onChange({
            ...element,
            content: { turns },
            isEmpty: turns.every((turn) => turn.text.trim() === ""),
          })
        }
      />
    );
  }

  if (element.type === "image_set" && "images" in element.content) {
    return (
      <p className={styles.readOnly}>
        {element.content.images.length === 0
          ? "No images are in this section yet."
          : "Images in this section are managed from the gallery."}
      </p>
    );
  }

  return null;
}

function ListEditor({
  items,
  pending,
  noun,
  onChange,
}: {
  items: TextItem[];
  pending: boolean;
  noun: string;
  onChange: (items: TextItem[]) => void;
}) {
  return (
    <div className={styles.listEditor}>
      {items.map((item, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Items stay ordered and hold no local state.
        <div className={styles.listItem} key={index}>
          <label>
            <span>
              Name <small>optional</small>
            </span>
            <input
              value={item.name ?? ""}
              onChange={(event) =>
                onChange(
                  items.map((current, itemIndex) =>
                    itemIndex === index
                      ? { ...current, name: event.target.value || undefined }
                      : current,
                  ),
                )
              }
              disabled={pending}
            />
          </label>
          <label className={styles.itemText}>
            <span>{capitalize(noun)}</span>
            <textarea
              rows={5}
              value={item.text}
              onChange={(event) =>
                onChange(
                  items.map((current, itemIndex) =>
                    itemIndex === index
                      ? { ...current, text: event.target.value }
                      : current,
                  ),
                )
              }
              disabled={pending}
            />
          </label>
          <button
            type="button"
            className={styles.removeItem}
            onClick={() =>
              onChange(items.filter((_, itemIndex) => itemIndex !== index))
            }
            disabled={pending}
          >
            <Trash2 size={14} aria-hidden="true" />
            Remove {noun}
          </button>
        </div>
      ))}
      <button
        type="button"
        className={styles.addItem}
        onClick={() => onChange([...items, { text: "" }])}
        disabled={pending}
      >
        <Plus size={16} aria-hidden="true" />
        Add {noun}
      </button>
    </div>
  );
}

function DialogueEditor({
  turns,
  pending,
  onChange,
}: {
  turns: DialogueTurn[];
  pending: boolean;
  onChange: (turns: DialogueTurn[]) => void;
}) {
  return (
    <div className={styles.listEditor}>
      {turns.map((turn, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Turns stay ordered and hold no local state.
        <div className={styles.listItem} key={index}>
          <label>
            <span>Speaker</span>
            <input
              value={turn.speaker}
              onChange={(event) =>
                onChange(
                  turns.map((current, turnIndex) =>
                    turnIndex === index
                      ? { ...current, speaker: event.target.value }
                      : current,
                  ),
                )
              }
              disabled={pending}
            />
          </label>
          <label className={styles.itemText}>
            <span>Message</span>
            <textarea
              rows={4}
              value={turn.text}
              onChange={(event) =>
                onChange(
                  turns.map((current, turnIndex) =>
                    turnIndex === index
                      ? { ...current, text: event.target.value }
                      : current,
                  ),
                )
              }
              disabled={pending}
            />
          </label>
          <button
            type="button"
            className={styles.removeItem}
            onClick={() =>
              onChange(turns.filter((_, turnIndex) => turnIndex !== index))
            }
            disabled={pending}
          >
            <Trash2 size={14} aria-hidden="true" />
            Remove turn
          </button>
        </div>
      ))}
      <button
        type="button"
        className={styles.addItem}
        onClick={() => onChange([...turns, { speaker: "", text: "" }])}
        disabled={pending}
      >
        <Plus size={16} aria-hidden="true" />
        Add turn
      </button>
    </div>
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
    content: element.content,
  };
}

function elementHint(type: AssetElement["type"]): string {
  switch (type) {
    case "prose":
      return "Write this at full width; it will keep the page’s reading layout.";
    case "text_set":
      return "Each greeting stays in this one ordered collection.";
    case "dialogue_sample":
      return "Keep each speaker and message together, in reading order.";
    case "image_set":
      return "This collection is presented here without exposing its stored data.";
  }
}

function capitalize(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
