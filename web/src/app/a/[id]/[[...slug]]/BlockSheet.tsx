"use client";

import { EyeOff, ImagePlus, Plus, RotateCcw, Trash2, X } from "lucide-react";
import Image from "next/image";
import { type FormEvent, useEffect, useRef, useState } from "react";
import {
  type AssetBlock,
  type AssetElement,
  type AssetImage,
  addAssetImage,
  type SaveAssetBlockRequest,
  saveAssetBlock,
} from "@/lib/api/query";
import { LAYOUTS } from "@/lib/page-arrangement";
import { LayoutPicker, WidthPicker } from "./ArrangementPickers";
import styles from "./BlockSheet.module.css";

type TextItem = { name?: string; text: string };
type DialogueTurn = { speaker: string; text: string };
type FieldItem = { name?: string; value: string };
type LinkItem = { label?: string; url: string; note?: string };
type ImageItem = { mediaId: string; name?: string };

export function BlockSheet({
  assetId,
  block,
  images,
  onDismiss,
  onSaved,
  onRemoved,
  onHide,
  onRemove,
  onImageAdded,
}: {
  assetId: string;
  block: AssetBlock;
  images: AssetImage[];
  onDismiss: () => void;
  onSaved: (block: AssetBlock) => void;
  onRemoved: () => void;
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
      if (saved) onSaved(saved);
      else onRemoved();
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

          <section
            className={styles.arrangement}
            aria-labelledby="section-arrangement"
          >
            <div className={styles.arrangementHeading}>
              <div>
                <h3 id="section-arrangement">Page arrangement</h3>
                <p>
                  Layout arranges this section’s content. Width places the
                  section on the desktop page.
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
              <h3 id="block-actions">Section actions</h3>
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
                            : "The section could not be hidden. Try again.",
                        ),
                      )
                      .finally(() => setPending(false));
                  }}
                >
                  <EyeOff size={16} aria-hidden="true" />
                  Hide section
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
                  Remove section
                </button>
              ) : null}
            </div>
          </section>

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
  assetId,
  element,
  images,
  pending,
  onChange,
  onRemove,
  onImageAdded,
}: {
  assetId: string;
  element: AssetElement;
  images: AssetImage[];
  pending: boolean;
  onChange: (element: AssetElement) => void;
  onRemove?: () => void;
  onImageAdded: () => void;
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
      <ElementFields
        assetId={assetId}
        element={element}
        images={images}
        pending={pending}
        onChange={onChange}
        onImageAdded={onImageAdded}
      />
    </section>
  );
}

function ElementFields({
  assetId,
  element,
  images,
  pending,
  onChange,
  onImageAdded,
}: {
  assetId: string;
  element: AssetElement;
  images: AssetImage[];
  pending: boolean;
  onChange: (element: AssetElement) => void;
  onImageAdded: () => void;
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

  if (element.type === "field_list" && "fields" in element.content) {
    return (
      <FieldEditor
        fields={element.content.fields}
        pending={pending}
        onChange={(fields) =>
          onChange({
            ...element,
            content: { fields },
            isEmpty: fields.every((field) => field.value.trim() === ""),
          })
        }
      />
    );
  }

  if (element.type === "link_list" && "links" in element.content) {
    return (
      <LinkEditor
        links={element.content.links}
        pending={pending}
        onChange={(links) =>
          onChange({
            ...element,
            content: { links },
            isEmpty: links.every((link) => link.url.trim() === ""),
          })
        }
      />
    );
  }

  if (element.type === "image_set" && "images" in element.content) {
    return (
      <ImageEditor
        assetId={assetId}
        items={element.content.images}
        images={images}
        pending={pending}
        onAdded={onImageAdded}
        onChange={(added) =>
          onChange({
            ...element,
            content: { images: added },
            isEmpty: added.length === 0,
          })
        }
      />
    );
  }

  return null;
}

function FieldEditor({
  fields,
  pending,
  onChange,
}: {
  fields: FieldItem[];
  pending: boolean;
  onChange: (fields: FieldItem[]) => void;
}) {
  return (
    <div className={styles.listEditor}>
      {fields.map((field, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Fields stay ordered and hold no local state.
        <div className={styles.fieldRow} key={index}>
          <label>
            <span>Name</span>
            <input
              value={field.name ?? ""}
              onChange={(event) =>
                onChange(
                  fields.map((current, fieldIndex) =>
                    fieldIndex === index
                      ? { ...current, name: event.target.value || undefined }
                      : current,
                  ),
                )
              }
              disabled={pending}
            />
          </label>
          <label>
            <span>Value</span>
            <input
              value={field.value}
              onChange={(event) =>
                onChange(
                  fields.map((current, fieldIndex) =>
                    fieldIndex === index
                      ? { ...current, value: event.target.value }
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
              onChange(fields.filter((_, fieldIndex) => fieldIndex !== index))
            }
            disabled={pending}
          >
            <Trash2 size={14} aria-hidden="true" />
            Remove field
          </button>
        </div>
      ))}
      <button
        type="button"
        className={styles.addItem}
        onClick={() => onChange([...fields, { value: "" }])}
        disabled={pending}
      >
        <Plus size={16} aria-hidden="true" />
        Add field
      </button>
    </div>
  );
}

function LinkEditor({
  links,
  pending,
  onChange,
}: {
  links: LinkItem[];
  pending: boolean;
  onChange: (links: LinkItem[]) => void;
}) {
  return (
    <div className={styles.listEditor}>
      {links.map((link, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Links stay ordered and hold no local state.
        <div className={styles.listItem} key={index}>
          <label>
            <span>
              Wording <small>optional</small>
            </span>
            <input
              value={link.label ?? ""}
              onChange={(event) =>
                onChange(
                  links.map((current, linkIndex) =>
                    linkIndex === index
                      ? { ...current, label: event.target.value || undefined }
                      : current,
                  ),
                )
              }
              disabled={pending}
            />
          </label>
          <label>
            <span>Address</span>
            <input
              type="url"
              inputMode="url"
              placeholder="https://"
              value={link.url}
              onChange={(event) =>
                onChange(
                  links.map((current, linkIndex) =>
                    linkIndex === index
                      ? { ...current, url: event.target.value }
                      : current,
                  ),
                )
              }
              disabled={pending}
            />
          </label>
          <label className={styles.itemText}>
            <span>
              Note <small>optional</small>
            </span>
            <input
              value={link.note ?? ""}
              onChange={(event) =>
                onChange(
                  links.map((current, linkIndex) =>
                    linkIndex === index
                      ? { ...current, note: event.target.value || undefined }
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
              onChange(links.filter((_, linkIndex) => linkIndex !== index))
            }
            disabled={pending}
          >
            <Trash2 size={14} aria-hidden="true" />
            Remove link
          </button>
        </div>
      ))}
      <button
        type="button"
        className={styles.addItem}
        onClick={() => onChange([...links, { url: "" }])}
        disabled={pending}
      >
        <Plus size={16} aria-hidden="true" />
        Add link
      </button>
    </div>
  );
}

function ImageEditor({
  assetId,
  items,
  images,
  pending,
  onChange,
  onAdded,
}: {
  assetId: string;
  items: ImageItem[];
  images: AssetImage[];
  pending: boolean;
  onChange: (items: ImageItem[]) => void;
  onAdded: () => void;
}) {
  const [uploading, setUploading] = useState(false);
  const [message, setMessage] = useState("");
  const [previews, setPreviews] = useState<Record<string, string>>({});
  const file = useRef<HTMLInputElement>(null);

  async function upload(chosen: File | null) {
    if (!chosen || uploading) return;
    setUploading(true);
    setMessage("");
    try {
      const mediaId = await addAssetImage(assetId, chosen);
      setPreviews((current) => ({
        ...current,
        [mediaId]: URL.createObjectURL(chosen),
      }));
      onChange([...items, { mediaId }]);
      onAdded();
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "The image could not be added. Try again.",
      );
    } finally {
      setUploading(false);
      if (file.current) file.current.value = "";
    }
  }

  return (
    <div className={styles.imageEditor}>
      {items.length > 0 ? (
        <ol className={styles.imageItems}>
          {items.map((item, index) => {
            const stored = images.find(
              (candidate) => candidate.id === item.mediaId,
            );
            const source = previews[item.mediaId] ?? stored?.thumbUrl;
            return (
              <li key={item.mediaId}>
                {source ? (
                  <Image
                    src={source}
                    alt=""
                    width={stored?.width ?? 200}
                    height={stored?.height ?? 200}
                    sizes="120px"
                    unoptimized
                  />
                ) : (
                  <span className={styles.imageMissing} />
                )}
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
                            ? {
                                ...current,
                                name: event.target.value || undefined,
                              }
                            : current,
                        ),
                      )
                    }
                    disabled={pending}
                  />
                </label>
                <div className={styles.imageMoves}>
                  <button
                    type="button"
                    disabled={pending || index === 0}
                    onClick={() => onChange(swap(items, index, index - 1))}
                  >
                    Earlier
                  </button>
                  <button
                    type="button"
                    disabled={pending || index === items.length - 1}
                    onClick={() => onChange(swap(items, index, index + 1))}
                  >
                    Later
                  </button>
                  <button
                    type="button"
                    className={styles.removeItem}
                    disabled={pending}
                    onClick={() =>
                      onChange(
                        items.filter((_, itemIndex) => itemIndex !== index),
                      )
                    }
                  >
                    <Trash2 size={14} aria-hidden="true" />
                    Remove
                  </button>
                </div>
              </li>
            );
          })}
        </ol>
      ) : (
        <p className={styles.readOnly}>No images are in this section yet.</p>
      )}
      {message ? (
        <p className={styles.imageError} role="alert">
          {message}
        </p>
      ) : null}
      <label className={styles.addItem}>
        <ImagePlus size={16} aria-hidden="true" />
        {uploading ? "Adding…" : "Add image"}
        <input
          ref={file}
          type="file"
          accept="image/*"
          className={styles.fileInput}
          disabled={pending || uploading}
          onChange={(event) => void upload(event.target.files?.[0] ?? null)}
        />
      </label>
    </div>
  );
}

function swap(items: ImageItem[], from: number, to: number): ImageItem[] {
  const next = [...items];
  [next[from], next[to]] = [next[to], next[from]];
  return next;
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
    itemSize: element.itemSize,
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
      return "Images sit in the order you put them in, and each may carry a name.";
    case "field_list":
      return "Each row is a short name and the value beside it.";
    case "link_list":
      return "Addresses have to start with http or https.";
  }
}

function capitalize(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
