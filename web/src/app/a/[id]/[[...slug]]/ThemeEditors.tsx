"use client";

import { FilePlus2, Plus, Trash2 } from "lucide-react";
import { useRef, useState } from "react";
import type {
  ColorSetContent,
  StylesheetSetContent,
  ThemeColor,
  ThemeColorMode,
  ThemeFile,
  ThemeStylesheet,
} from "@/lib/api/query";
import { pickerColor } from "@/lib/theme-colors";
import { replaceAt, without } from "./CollectionEditor";
import styles from "./ThemeEditors.module.css";

export function ColorSetEditor({
  content,
  pending,
  onChange,
}: {
  content: ColorSetContent;
  pending: boolean;
  onChange: (content: ColorSetContent) => void;
}) {
  function changeMode(index: number, mode: ThemeColorMode) {
    onChange({ modes: replaceAt(content.modes, index, mode) });
  }

  return (
    <div className={styles.paletteEditor}>
      {content.modes.map((mode, modeIndex) => (
        <section className={styles.mode} key={mode.name || modeIndex}>
          <div className={styles.modeHeading}>
            <label>
              <span>Mode</span>
              <input
                value={mode.name ?? ""}
                placeholder="Default"
                onChange={(event) =>
                  changeMode(modeIndex, {
                    ...mode,
                    name: event.target.value || undefined,
                  })
                }
                disabled={pending}
              />
            </label>
            {content.modes.length > 1 ? (
              <button
                type="button"
                className={styles.remove}
                onClick={() =>
                  onChange({ modes: without(content.modes, modeIndex) })
                }
                disabled={pending}
              >
                <Trash2 size={14} aria-hidden="true" />
                Remove mode
              </button>
            ) : null}
          </div>
          <ColorRows
            colors={mode.colors}
            pending={pending}
            onChange={(colors) => changeMode(modeIndex, { ...mode, colors })}
          />
        </section>
      ))}
      <button
        type="button"
        className={styles.add}
        onClick={() =>
          onChange({
            modes: [
              ...content.modes,
              { name: `Mode ${content.modes.length + 1}`, colors: [] },
            ],
          })
        }
        disabled={pending}
      >
        <Plus size={16} aria-hidden="true" />
        Add mode
      </button>
    </div>
  );
}

function ColorRows({
  colors,
  pending,
  onChange,
}: {
  colors: ThemeColor[];
  pending: boolean;
  onChange: (colors: ThemeColor[]) => void;
}) {
  return (
    <div className={styles.colorRows}>
      {colors.map((color, index) => {
        const pickerValue = pickerColor(color.value);
        return (
          <div className={styles.colorRow} key={color.id ?? index}>
            <label
              className={styles.colorPicker}
              style={{ backgroundColor: color.value }}
            >
              <span className={styles.srOnly}>Choose {color.name} colour</span>
              <input
                type="color"
                value={pickerValue}
                onChange={(event) =>
                  onChange(
                    replaceAt(colors, index, { value: event.target.value }),
                  )
                }
                disabled={pending}
              />
            </label>
            <label>
              <span>Name</span>
              <input
                value={color.name}
                onChange={(event) =>
                  onChange(
                    replaceAt(colors, index, { name: event.target.value }),
                  )
                }
                disabled={pending}
              />
            </label>
            <label>
              <span>Colour</span>
              <input
                value={color.value}
                placeholder="#7c5cff or rgba(124, 92, 255, 1)"
                onChange={(event) =>
                  onChange(
                    replaceAt(colors, index, { value: event.target.value }),
                  )
                }
                disabled={pending}
              />
            </label>
            <button
              type="button"
              className={styles.iconRemove}
              aria-label={`Remove ${color.name || `colour ${index + 1}`}`}
              onClick={() => onChange(without(colors, index))}
              disabled={pending}
            >
              <Trash2 size={15} aria-hidden="true" />
            </button>
          </div>
        );
      })}
      <button
        type="button"
        className={styles.add}
        onClick={() =>
          onChange([
            ...colors,
            { id: crypto.randomUUID(), name: "colour", value: "#7c5cff" },
          ])
        }
        disabled={pending}
      >
        <Plus size={16} aria-hidden="true" />
        Add colour
      </button>
    </div>
  );
}

export function StylesheetSetEditor({
  content,
  pending,
  onChange,
}: {
  content: StylesheetSetContent;
  pending: boolean;
  onChange: (content: StylesheetSetContent) => void;
}) {
  const [message, setMessage] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);

  async function addFiles(files: FileList | null) {
    if (!files?.length) return;
    setMessage("");
    try {
      const attached = await Promise.all(Array.from(files, themeFile));
      onChange({
        ...content,
        assets: [...(content.assets ?? []), ...attached],
      });
    } catch {
      setMessage("Those files could not be attached. Try them again.");
    } finally {
      if (fileInput.current) fileInput.current.value = "";
    }
  }

  return (
    <div className={styles.stylesEditor}>
      <label className={styles.codeField}>
        <span>Main stylesheet</span>
        <textarea
          rows={12}
          spellCheck={false}
          value={content.global}
          onChange={(event) =>
            onChange({ ...content, global: event.target.value })
          }
          disabled={pending}
        />
      </label>

      <div className={styles.componentSheets}>
        {(content.stylesheets ?? []).map((sheet, index) => (
          <ComponentSheet
            key={sheet.id ?? index}
            sheet={sheet}
            position={index}
            pending={pending}
            onChange={(changes) =>
              onChange({
                ...content,
                stylesheets: replaceAt(
                  content.stylesheets ?? [],
                  index,
                  changes,
                ),
              })
            }
            onRemove={() =>
              onChange({
                ...content,
                stylesheets: without(content.stylesheets ?? [], index),
              })
            }
          />
        ))}
        <button
          type="button"
          className={styles.add}
          onClick={() =>
            onChange({
              ...content,
              stylesheets: [
                ...(content.stylesheets ?? []),
                {
                  id: crypto.randomUUID(),
                  name: `Component ${(content.stylesheets ?? []).length + 1}`,
                  css: "",
                  enabled: true,
                },
              ],
            })
          }
          disabled={pending}
        >
          <Plus size={16} aria-hidden="true" />
          Add component stylesheet
        </button>
      </div>

      <section className={styles.files}>
        <div className={styles.filesHeading}>
          <div>
            <h4>Theme files</h4>
            <p>
              Fonts and other files referenced by the CSS stay inside the theme
              bundle.
            </p>
          </div>
          <label className={styles.addFile}>
            <FilePlus2 size={16} aria-hidden="true" />
            Attach files
            <input
              ref={fileInput}
              type="file"
              multiple
              accept="font/*,.woff,.woff2,.ttf,.otf"
              onChange={(event) => void addFiles(event.target.files)}
              disabled={pending}
            />
          </label>
        </div>
        {(content.assets ?? []).length > 0 ? (
          <ul>
            {(content.assets ?? []).map((asset, index) => (
              <li key={asset.id ?? index}>
                <span>{asset.path}</span>
                <small>{asset.mediaType || "Attached file"}</small>
                <button
                  type="button"
                  onClick={() =>
                    onChange({
                      ...content,
                      assets: without(content.assets ?? [], index),
                    })
                  }
                  disabled={pending}
                >
                  <Trash2 size={14} aria-hidden="true" />
                  Remove
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <p className={styles.noFiles}>No files attached.</p>
        )}
        {message ? (
          <p role="alert" className={styles.error}>
            {message}
          </p>
        ) : null}
      </section>
    </div>
  );
}

function ComponentSheet({
  sheet,
  position,
  pending,
  onChange,
  onRemove,
}: {
  sheet: ThemeStylesheet;
  position: number;
  pending: boolean;
  onChange: (changes: Partial<ThemeStylesheet>) => void;
  onRemove: () => void;
}) {
  return (
    <section
      className={styles.componentSheet}
      data-off={!sheet.enabled || undefined}
    >
      <div className={styles.sheetHeading}>
        <label>
          <span>Name</span>
          <input
            value={sheet.name}
            onChange={(event) => onChange({ name: event.target.value })}
            disabled={pending}
          />
        </label>
        <label className={styles.enabled}>
          <input
            type="checkbox"
            checked={sheet.enabled}
            onChange={(event) => onChange({ enabled: event.target.checked })}
            disabled={pending}
          />
          Included
        </label>
        <button
          type="button"
          className={styles.remove}
          onClick={onRemove}
          disabled={pending}
        >
          <Trash2 size={14} aria-hidden="true" />
          Remove
        </button>
      </div>
      <label className={styles.codeField}>
        <span>{sheet.name || `Component ${position + 1}`} CSS</span>
        <textarea
          rows={8}
          spellCheck={false}
          value={sheet.css}
          onChange={(event) => onChange({ css: event.target.value })}
          disabled={pending}
        />
      </label>
    </section>
  );
}

async function themeFile(file: File): Promise<ThemeFile> {
  const bytes = new Uint8Array(await file.arrayBuffer());
  let binary = "";
  for (let start = 0; start < bytes.length; start += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(start, start + 0x8000));
  }
  return {
    id: crypto.randomUUID(),
    path: `assets/${file.name.replaceAll("\\", "-")}`,
    mediaType: file.type || undefined,
    data: btoa(binary),
  };
}
