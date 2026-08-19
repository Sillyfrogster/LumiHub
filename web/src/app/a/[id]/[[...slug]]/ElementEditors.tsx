"use client";

import { ImagePlus, Maximize2, Plus, Trash2 } from "lucide-react";
import Image from "next/image";
import { useRef, useState } from "react";
import {
  type AssetElement,
  type AssetImage,
  addAssetImage,
} from "@/lib/api/query";
import { fitsInTheSheet, opensFullScreen } from "@/lib/page-arrangement";
import styles from "./BlockSheet.module.css";
import { replaceAt, without } from "./CollectionEditor";
import { EntryTableEditor } from "./EntryTableEditor";
import {
  PromptListEditor,
  ScriptListEditor,
  SettingGroupEditor,
  VariableSchemaEditor,
} from "./PresetEditors";

type TextItem = { name?: string; text: string };
type DialogueTurn = { speaker: string; text: string };
type FieldItem = { name?: string; value: string };
type LinkItem = { label?: string; url: string; note?: string };
type ImageItem = { mediaId: string; name?: string };

/**
 * One element inside a section sheet. Small content is edited here. Content
 * past what a sheet can hold says so and sends the creator to the full-screen
 * surface, which stays available either way.
 */
export function ElementEditor({
  assetId,
  element,
  images,
  pending,
  onChange,
  onRemove,
  onImageAdded,
  onExpand,
}: {
  assetId: string;
  element: AssetElement;
  images: AssetImage[];
  pending: boolean;
  onChange: (element: AssetElement) => void;
  onRemove?: () => void;
  onImageAdded: () => void;
  onExpand?: () => void;
}) {
  const editableHere = fitsInTheSheet(element);
  return (
    <section
      className={styles.element}
      aria-labelledby={`element-${element.id}`}
    >
      <div className={styles.elementHeading}>
        <div>
          <h3 id={`element-${element.id}`}>{element.label || "Content"}</h3>
          <p>{elementHint(element.type)}</p>
          {element.facts.length > 0 ? (
            <p className={styles.elementFacts}>{element.facts.join(" · ")}</p>
          ) : null}
        </div>
        <div className={styles.elementActions}>
          {onExpand && opensFullScreen(element.type) ? (
            <button
              type="button"
              className={styles.expand}
              onClick={onExpand}
              disabled={pending}
            >
              <Maximize2 size={15} aria-hidden="true" />
              Full screen
            </button>
          ) : null}
          {onRemove ? (
            <button type="button" onClick={onRemove} disabled={pending}>
              <Trash2 size={15} aria-hidden="true" />
              Remove
            </button>
          ) : null}
        </div>
      </div>
      {editableHere ? (
        <ElementFields
          assetId={assetId}
          element={element}
          images={images}
          pending={pending}
          onChange={onChange}
          onImageAdded={onImageAdded}
        />
      ) : (
        <p className={styles.readOnly}>
          There is more here than a sheet can hold. Full screen is where it is
          edited, and nothing about it changes on the way.
        </p>
      )}
    </section>
  );
}

/** The editing fields for one element, at whatever width it is given. */
export function ElementFields({
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
        noun={element.role === "prompt_nudges" ? "nudge" : "greeting"}
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
        // Putting an image in an expression set is what identifies it as one.
        mediaRole={element.role === "expressions" ? "expression" : "gallery"}
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

  if (element.type === "entry_table" && "entries" in element.content) {
    return (
      <EntryTableEditor
        entries={element.content.entries}
        pending={pending}
        onChange={(entries) =>
          onChange({
            ...element,
            content: { entries },
            isEmpty: entries.every((entry) => entry.text.trim() === ""),
          })
        }
      />
    );
  }

  if (element.type === "prompt_list" && "fragments" in element.content) {
    return (
      <PromptListEditor
        content={element.content}
        pending={pending}
        onChange={(content) =>
          onChange({
            ...element,
            content,
            isEmpty: content.fragments.length === 0,
          })
        }
      />
    );
  }

  if (element.type === "setting_group" && "settings" in element.content) {
    return (
      <SettingGroupEditor
        settings={element.content.settings}
        pending={pending}
        onChange={(settings) =>
          onChange({
            ...element,
            content: { settings },
            isEmpty: settings.every((setting) => setting.value == null),
          })
        }
      />
    );
  }

  if (element.type === "variable_schema" && "variables" in element.content) {
    return (
      <VariableSchemaEditor
        variables={element.content.variables}
        pending={pending}
        onChange={(variables) =>
          onChange({
            ...element,
            content: { variables },
            isEmpty: variables.length === 0,
          })
        }
      />
    );
  }

  if (element.type === "script_list" && "scripts" in element.content) {
    return (
      <ScriptListEditor
        scripts={element.content.scripts}
        pending={pending}
        onChange={(scripts) =>
          onChange({
            ...element,
            content: { scripts },
            isEmpty: scripts.length === 0,
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
                  replaceAt(fields, index, {
                    name: event.target.value || undefined,
                  }),
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
                  replaceAt(fields, index, { value: event.target.value }),
                )
              }
              disabled={pending}
            />
          </label>
          <button
            type="button"
            className={styles.removeItem}
            onClick={() => onChange(without(fields, index))}
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
                  replaceAt(links, index, {
                    label: event.target.value || undefined,
                  }),
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
                onChange(replaceAt(links, index, { url: event.target.value }))
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
                  replaceAt(links, index, {
                    note: event.target.value || undefined,
                  }),
                )
              }
              disabled={pending}
            />
          </label>
          <button
            type="button"
            className={styles.removeItem}
            onClick={() => onChange(without(links, index))}
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
  mediaRole,
  onChange,
  onAdded,
}: {
  assetId: string;
  items: ImageItem[];
  images: AssetImage[];
  pending: boolean;
  mediaRole: "expression" | "gallery";
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
      const mediaId = await addAssetImage(assetId, chosen, mediaRole);
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
                        replaceAt(items, index, {
                          name: event.target.value || undefined,
                        }),
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
                    onClick={() => onChange(without(items, index))}
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
                  replaceAt(items, index, {
                    name: event.target.value || undefined,
                  }),
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
                onChange(replaceAt(items, index, { text: event.target.value }))
              }
              disabled={pending}
            />
          </label>
          <button
            type="button"
            className={styles.removeItem}
            onClick={() => onChange(without(items, index))}
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
                  replaceAt(turns, index, { speaker: event.target.value }),
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
                onChange(replaceAt(turns, index, { text: event.target.value }))
              }
              disabled={pending}
            />
          </label>
          <button
            type="button"
            className={styles.removeItem}
            onClick={() => onChange(without(turns, index))}
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
    case "entry_table":
      return "Each entry is switched on by its own keys.";
    case "prompt_list":
      return "Fragments are sent in the order they sit in, under the headings you give them.";
    case "setting_group":
      return "The names are your app's own, and a setting you leave out stays out of the file.";
    case "variable_schema":
      return "Each variable is one thing a reader chooses before the preset runs.";
    case "script_list":
      return "Each script finds something and writes something else in its place.";
  }
}
function capitalize(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
