import type { AssetBlock } from "@/lib/api/query";
import styles from "./AssetBlocks.module.css";
import { ElementBody } from "./ElementBody";

/** One row arrangement per layout preset the catalog can choose. */
const LAYOUT_CLASS: Record<AssetBlock["layout"], string> = {
  single: styles.single,
  duo: styles.duo,
  "main-aside": styles.mainAside,
  trio: styles.trio,
  "stack-2": styles.stack,
  "stack-3": styles.stack,
};

/**
 * The asset's content, in page order. A reader is shown the blocks that carry
 * something. The owner is shown every block the kind asks for, because an
 * empty required block is what says what is still theirs to write.
 */
export function AssetBlocks({
  blocks,
  isOwner,
}: {
  blocks: AssetBlock[];
  isOwner: boolean;
}) {
  const shown = isOwner
    ? blocks
    : blocks.filter((block) => !block.hidden && !block.isEmpty);
  if (shown.length === 0) return null;

  return (
    <div className={styles.column}>
      {shown.map((block) => (
        <article key={block.id} className={styles.block}>
          <h2 className={styles.title}>{block.title}</h2>
          <div className={LAYOUT_CLASS[block.layout]}>
            {block.elements.map((element) => (
              <ElementBody
                key={element.id}
                element={element}
                isOwner={isOwner}
              />
            ))}
          </div>
        </article>
      ))}
    </div>
  );
}
