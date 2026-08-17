import type { AssetElement } from "@/lib/api/query";
import styles from "./ElementBody.module.css";

/**
 * One element on the page. An empty one renders nothing to a reader and a
 * labelled placeholder to its owner.
 */
export function ElementBody({
  element,
  isOwner,
}: {
  element: AssetElement;
  isOwner: boolean;
}) {
  if (element.isEmpty && !isOwner) return null;

  return (
    <section className={styles.element}>
      {element.label ? <h3 className={styles.label}>{element.label}</h3> : null}
      {element.isEmpty ? (
        <p className={styles.blank}>Nothing written here yet.</p>
      ) : (
        <ElementContent element={element} />
      )}
    </section>
  );
}

function ElementContent({ element }: { element: AssetElement }) {
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

  // Image sets arrive with the gallery block, which nothing fills yet.
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
