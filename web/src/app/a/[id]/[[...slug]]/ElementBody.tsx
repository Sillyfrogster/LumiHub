"use client";

import { Maximize2 } from "lucide-react";
import Image from "next/image";
import type { CSSProperties } from "react";
import type { AssetElement, AssetImage, LorebookEntry } from "@/lib/api/query";
import { fitsInTheSheet, opensFullScreen } from "@/lib/page-arrangement";
import styles from "./ElementBody.module.css";

/** The image sizes an element can draw its items at, as a rendered width. */
const ITEM_WIDTHS = { small: "120px", medium: "180px", large: "260px" };

/**
 * One element on the page. An empty one renders nothing to a reader and a
 * labelled placeholder to its owner. A heading appears only where it says
 * something the section's own title does not, because a section holding one
 * element is already named by that title.
 */
export function ElementBody({
  element,
  isOwner,
  images = [],
  blockTitle,
  onExpand,
}: {
  element: AssetElement;
  isOwner: boolean;
  images?: AssetImage[];
  blockTitle?: string;
  onExpand?: () => void;
}) {
  if (element.isEmpty && !isOwner) return null;

  const label =
    element.role && element.label && element.label !== blockTitle
      ? element.label
      : null;
  // Size earns a line on the page once the content is past a glance.
  const facts = fitsInTheSheet(element) ? [] : element.facts;
  const expandable = isOwner && onExpand && opensFullScreen(element.type);

  return (
    <section className={styles.element}>
      {label || facts.length > 0 || expandable ? (
        <div className={styles.heading}>
          <div>
            {label ? <h3 className={styles.label}>{label}</h3> : null}
            {facts.length > 0 ? (
              <p className={styles.facts}>{facts.join(" · ")}</p>
            ) : null}
          </div>
          {expandable ? (
            <button
              type="button"
              className={styles.expand}
              onClick={onExpand}
              aria-label={`Edit ${element.label || "this content"} in full screen`}
            >
              <Maximize2 size={14} aria-hidden="true" />
              Edit in full screen
            </button>
          ) : null}
        </div>
      ) : null}
      {element.isEmpty ? (
        <p className={styles.blank}>Nothing written here yet.</p>
      ) : (
        <ElementContent element={element} images={images} />
      )}
    </section>
  );
}

function ElementContent({
  element,
  images,
}: {
  element: AssetElement;
  images: AssetImage[];
}) {
  const { content } = element;

  if (element.type === "prose" && "text" in content) {
    return element.display === "verbatim" ? (
      <pre className={styles.verbatim}>{content.text}</pre>
    ) : (
      <Paragraphs text={content.text} />
    );
  }

  if (element.type === "text_set" && "texts" in content) {
    return (
      <ol className={styles.textSet}>
        {content.texts.map((item, index) => (
          <li key={`${index}-${item.name ?? ""}`}>
            {item.name ? <p className={styles.itemName}>{item.name}</p> : null}
            <Paragraphs text={item.text} />
          </li>
        ))}
      </ol>
    );
  }

  if (element.type === "dialogue_sample" && "turns" in content) {
    return (
      <ol className={styles.dialogue}>
        {content.turns.map((turn, index) => (
          <li key={`${index}-${turn.speaker}`}>
            <p className={styles.speaker}>{turn.speaker}</p>
            <Paragraphs text={turn.text} />
          </li>
        ))}
      </ol>
    );
  }

  if (element.type === "field_list" && "fields" in content) {
    return (
      <dl className={styles.fieldList}>
        {content.fields.map((field, index) => (
          <div key={`${index}-${field.name ?? ""}`}>
            <dt>{field.name || "Unnamed"}</dt>
            <dd>{field.value}</dd>
          </div>
        ))}
      </dl>
    );
  }

  if (element.type === "link_list" && "links" in content) {
    return (
      <ul className={styles.linkList}>
        {content.links.map((link, index) => (
          <li key={`${index}-${link.url}`}>
            <a href={link.url} rel="noreferrer nofollow" target="_blank">
              {link.label || link.url}
            </a>
            {link.note ? <span>{link.note}</span> : null}
          </li>
        ))}
      </ul>
    );
  }

  if (element.type === "image_set" && "images" in content) {
    const width = ITEM_WIDTHS[element.itemSize ?? "medium"];
    return (
      <ul
        className={styles.imageSet}
        style={{ "--item-width": width } as CSSProperties}
      >
        {content.images.map((item) => {
          const image = images.find(
            (candidate) => candidate.id === item.mediaId,
          );
          if (!image) return null;
          return (
            <li key={item.mediaId}>
              <Image
                src={image.thumbUrl}
                alt={item.name || ""}
                width={image.width}
                height={image.height}
                sizes="260px"
                unoptimized
              />
              {item.name ? <span>{item.name}</span> : null}
            </li>
          );
        })}
      </ul>
    );
  }

  if (element.type === "entry_table" && "entries" in content) {
    return <EntryTable entries={content.entries} />;
  }

  return null;
}

/**
 * A book as its entries. Four columns where the section is wide enough to hold
 * them, and a stacked list where it is not, because four columns in 430 pixels
 * is not a table anybody can read.
 */
function EntryTable({ entries }: { entries: LorebookEntry[] }) {
  return (
    <div className={styles.entryTableScroll}>
      <table className={styles.entryTable}>
        <thead>
          <tr>
            <th scope="col">Entry</th>
            <th scope="col">Keys</th>
            <th scope="col">Text</th>
            <th scope="col">State</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: Entries stay ordered and hold no local state.
            <tr key={index} data-off={entry.enabled ? undefined : true}>
              <td data-column="Entry">
                <span className={styles.entryName}>
                  {entry.name?.trim() || `Entry ${index + 1}`}
                </span>
              </td>
              <td data-column="Keys">
                {entry.keys.length === 0 ? (
                  <span className={styles.entryNoKeys}>
                    {entry.constant ? "Always on" : "No keys"}
                  </span>
                ) : (
                  <ul className={styles.entryKeys}>
                    {entry.keys.map((key) => (
                      <li key={key}>{key}</li>
                    ))}
                  </ul>
                )}
              </td>
              <td data-column="Text">
                <Paragraphs text={entry.text} />
              </td>
              <td data-column="State">{entry.enabled ? "On" : "Off"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Paragraphs({ text }: { text: string }) {
  const paragraphs = text.split(/\n{2,}/).filter((line) => line.trim() !== "");
  return (
    <>
      {paragraphs.map((paragraph, index) => (
        <p key={`${index}-${paragraph.slice(0, 24)}`}>{paragraph}</p>
      ))}
    </>
  );
}
