"use client";

import { Eye, ListTree, Plus, Undo2, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AssetBlock } from "@/lib/api/query";
import { useAuth } from "@/lib/auth";
import styles from "./ContentsBar.module.css";
import { CreatorMenu, type CreatorMenuProps } from "./CreatorMenu";

type ContentsBlock = Pick<AssetBlock, "id" | "title">;

export function ContentsBar({
  blocks,
  isOwner,
  arranging,
  adding,
  readerView,
  canAdd,
  creatorMenu,
  onToggleArrange,
  onToggleAdd,
  onReaderView,
  onReturnToEditing,
}: {
  blocks: ContentsBlock[];
  isOwner: boolean;
  arranging: boolean;
  adding: boolean;
  readerView: boolean;
  canAdd: boolean;
  creatorMenu: CreatorMenuProps;
  onToggleArrange: () => void;
  onToggleAdd: () => void;
  onReaderView: () => void;
  onReturnToEditing: () => void;
}) {
  const { account } = useAuth();
  const bar = useRef<HTMLDivElement>(null);
  const [activeBlockId, setActiveBlockId] = useState(blocks[0]?.id ?? null);
  const hasStaffTools = Boolean(
    account?.role === "admin" && !creatorMenu.isDraft && !creatorMenu.withheld,
  );
  const hasPageTools = isOwner || hasStaffTools;

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

  if (blocks.length === 0 && !hasPageTools) return null;

  return (
    <div ref={bar} className={styles.bar}>
      {arranging ? (
        <p className={styles.mode}>Arrangement outline</p>
      ) : blocks.length > 0 ? (
        <nav className={styles.contents} aria-label="Contents">
          <span className={styles.label}>Contents</span>
          <ol className={styles.entries}>
            {blocks.map((block) => (
              <li key={block.id}>
                <a
                  href={`#block-${block.id}`}
                  aria-current={
                    activeBlockId === block.id ? "location" : undefined
                  }
                  onClick={() => setActiveBlockId(block.id)}
                >
                  {block.title}
                </a>
              </li>
            ))}
          </ol>
        </nav>
      ) : isOwner ? (
        <p className={styles.mode}>Start this page</p>
      ) : null}

      {hasPageTools ? (
        <div className={styles.tools} role="toolbar" aria-label="Page tools">
          {isOwner && readerView ? (
            <>
              <span className={styles.viewing}>
                <Eye size={16} aria-hidden="true" />
                <span>Reader’s view</span>
              </span>
              <button type="button" onClick={onReturnToEditing}>
                <Undo2 size={16} aria-hidden="true" />
                <span>Return</span>
              </button>
            </>
          ) : isOwner ? (
            <>
              <button
                type="button"
                aria-expanded={arranging}
                onClick={onToggleArrange}
              >
                {arranging ? (
                  <X size={17} aria-hidden="true" />
                ) : (
                  <ListTree size={17} aria-hidden="true" />
                )}
                <span>{arranging ? "Close outline" : "Arrange"}</span>
              </button>
              {canAdd ? (
                <button
                  type="button"
                  aria-expanded={adding}
                  onClick={onToggleAdd}
                >
                  {adding ? (
                    <X size={17} aria-hidden="true" />
                  ) : (
                    <Plus size={17} aria-hidden="true" />
                  )}
                  <span>{adding ? "Close add" : "Add block"}</span>
                </button>
              ) : null}
              <button type="button" onClick={onReaderView}>
                <Eye size={17} aria-hidden="true" />
                <span>Reader’s view</span>
              </button>
              <CreatorMenu {...creatorMenu} />
            </>
          ) : (
            <CreatorMenu {...creatorMenu} />
          )}
        </div>
      ) : null}
    </div>
  );
}
