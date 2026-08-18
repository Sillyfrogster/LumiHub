import Image from "next/image";
import type { CSSProperties } from "react";
import type { AssetElement, AssetImage } from "@/lib/api/query";
import styles from "./ElementBody.module.css";

/** The image sizes an element can draw its items at, as a rendered width. */
const ITEM_WIDTHS = { small: "120px", medium: "180px", large: "260px" };

/**
 * One element on the page. An empty one renders nothing to a reader and a
 * labelled placeholder to its owner. A heading appears only where the element
 * carries a role, because a section holding one nameless element is already
 * named by its own title.
 */
export function ElementBody({
  element,
  isOwner,
  images = [],
}: {
  element: AssetElement;
  isOwner: boolean;
  images?: AssetImage[];
}) {
  if (element.isEmpty && !isOwner) return null;

  return (
    <section className={styles.element}>
      {element.role && element.label ? (
        <h3 className={styles.label}>{element.label}</h3>
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

  return null;
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
