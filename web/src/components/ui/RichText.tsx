import { type RichBlock, type RichInline, readRichText } from "@/lib/rich-text";
import styles from "./RichText.module.css";

export function RichText({
  text,
  className,
}: {
  text: string;
  className?: string;
}) {
  const { blocks } = readRichText(text);
  if (blocks.length === 0) return null;
  return (
    <div className={className ? `${styles.rich} ${className}` : styles.rich}>
      <Blocks blocks={blocks} />
    </div>
  );
}

export function FormattingNotice() {
  return (
    <small className={styles.notice}>
      The page shows the words, not the formatting written into this text. The
      download is unchanged.
    </small>
  );
}

function Blocks({ blocks }: { blocks: RichBlock[] }) {
  return (
    <>
      {blocks.map((block, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Blocks follow the writing and hold no local state.
        <Block block={block} key={index} />
      ))}
    </>
  );
}

function Block({ block }: { block: RichBlock }) {
  if (block.kind === "paragraph") {
    return (
      <p>
        <Inline nodes={block.children} />
      </p>
    );
  }

  if (block.kind === "heading") {
    /** Page headings already occupy h1 through h3. */
    const Tag = `h${Math.min(block.depth + 3, 6)}` as "h4" | "h5" | "h6";
    return (
      <Tag className={styles.heading}>
        <Inline nodes={block.children} />
      </Tag>
    );
  }

  if (block.kind === "quote") {
    return (
      <blockquote className={styles.quote}>
        <Blocks blocks={block.children} />
      </blockquote>
    );
  }

  const Tag = block.ordered ? "ol" : "ul";
  return (
    <Tag
      className={styles.list}
      start={block.ordered ? block.start : undefined}
    >
      {block.items.map((item, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Items follow the writing and hold no local state.
        <li key={index}>
          <Blocks blocks={item} />
        </li>
      ))}
    </Tag>
  );
}

function Inline({ nodes }: { nodes: RichInline[] }) {
  return (
    <>
      {nodes.map((node, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Runs follow the writing and hold no local state.
        <InlineNode node={node} key={index} />
      ))}
    </>
  );
}

function InlineNode({ node }: { node: RichInline }) {
  switch (node.kind) {
    case "text":
      return <>{node.text}</>;
    case "break":
      return <br />;
    case "code":
      return <code className={styles.code}>{node.text}</code>;
    case "emphasis":
      return (
        <em>
          <Inline nodes={node.children} />
        </em>
      );
    case "strong":
      return (
        <strong>
          <Inline nodes={node.children} />
        </strong>
      );
    default: {
      const away = !node.href.startsWith("/");
      return (
        <a
          className={styles.link}
          href={node.href}
          rel="noreferrer nofollow"
          target={away ? "_blank" : undefined}
        >
          <Inline nodes={node.children} />
        </a>
      );
    }
  }
}
