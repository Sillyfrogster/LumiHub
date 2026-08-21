"use client";

import { useEffect, useRef, useState } from "react";
import type { AssetBlock } from "@/lib/api/query";
import styles from "./ContentsBar.module.css";

type ContentsBlock = Pick<AssetBlock, "id" | "title">;

export function ContentsBar({ blocks }: { blocks: ContentsBlock[] }) {
  const bar = useRef<HTMLElement>(null);
  const [activeBlockId, setActiveBlockId] = useState(blocks[0]?.id ?? null);

  useEffect(() => {
    const hash = window.location.hash;
    const hashBlockId = hash.startsWith("#block-")
      ? hash.slice("#block-".length)
      : null;
    setActiveBlockId((active) => {
      if (hashBlockId && blocks.some((block) => block.id === hashBlockId)) {
        return hashBlockId;
      }
      return active && blocks.some((block) => block.id === active)
        ? active
        : (blocks[0]?.id ?? null);
    });
  }, [blocks]);

  useEffect(() => {
    if (blocks.length === 0) return;

    let frame = 0;
    const updateActiveBlock = () => {
      frame = 0;
      const readingLine =
        (bar.current?.getBoundingClientRect().bottom ?? 0) + 16;
      const current = blocks.find((block) => {
        const target = document.getElementById(`block-${block.id}`);
        return target
          ? target.getBoundingClientRect().bottom > readingLine
          : false;
      });
      setActiveBlockId((active) => current?.id ?? blocks.at(-1)?.id ?? active);
    };
    const scheduleUpdate = () => {
      if (frame) return;
      frame = window.requestAnimationFrame(updateActiveBlock);
    };

    scheduleUpdate();
    window.addEventListener("scroll", scheduleUpdate, { passive: true });
    window.addEventListener("resize", scheduleUpdate);
    return () => {
      window.removeEventListener("scroll", scheduleUpdate);
      window.removeEventListener("resize", scheduleUpdate);
      if (frame) window.cancelAnimationFrame(frame);
    };
  }, [blocks]);

  if (blocks.length === 0) return null;

  return (
    <nav ref={bar} className={styles.bar} aria-label="Contents">
      <span className={styles.label}>Contents</span>
      <ol className={styles.entries}>
        {blocks.map((block) => (
          <li key={block.id}>
            <a
              href={`#block-${block.id}`}
              aria-current={activeBlockId === block.id ? "location" : undefined}
              onClick={() => setActiveBlockId(block.id)}
            >
              {block.title}
            </a>
          </li>
        ))}
      </ol>
    </nav>
  );
}
