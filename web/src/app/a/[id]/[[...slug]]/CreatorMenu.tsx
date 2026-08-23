"use client";

import { Settings2, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { AssetDetail, ReadinessItem } from "@/lib/api/query";
import { useAuth } from "@/lib/auth";
import styles from "./CreatorMenu.module.css";
import { OPEN_CREATOR_MENU } from "./creator-menu";
import { DeleteControl } from "./DeleteControl";
import { DiscoveryControl } from "./DiscoveryControl";
import { IdentityPanel } from "./IdentityPanel";
import { PreservedPanel } from "./PreservedPanel";
import { PublishPanel } from "./PublishPanel";
import { SealedPanel } from "./SealedPanel";
import { ShortfallPanel } from "./ShortfallPanel";
import { WithholdControl } from "./WithholdControl";

export type CreatorMenuProps = {
  assetId: string;
  creator: string;
  kind: string;
  name: string;
  isNsfw: boolean | null;
  isDraft: boolean;
  isOwner: boolean;
  discovery: AssetDetail["discovery"];
  withheld: boolean;
  hasOriginal: boolean;
  readiness?: ReadinessItem[];
  sealedBlocks?: number;
};

export function CreatorMenu(props: CreatorMenuProps) {
  const { account } = useAuth();
  const dialog = useRef<HTMLDialogElement>(null);
  const [mounted, setMounted] = useState(false);
  const [open, setOpen] = useState(false);
  const isAdmin = account?.role === "admin";
  const canWithhold = Boolean(isAdmin && !props.isDraft && !props.withheld);
  const canOpenMenu = props.isOwner || canWithhold;
  const missing = props.readiness?.filter((item) => !item.met).length ?? 0;

  const show = useCallback(() => {
    if (!canOpenMenu || dialog.current?.open) return;
    dialog.current?.showModal();
    setOpen(true);
  }, [canOpenMenu]);

  function close() {
    dialog.current?.close();
    setOpen(false);
  }

  function goToBlock(blockId: string) {
    close();
    window.requestAnimationFrame(() => {
      const target = document.getElementById(`block-${blockId}`);
      if (!target) return;
      target.scrollIntoView({ block: "start" });
      window.history.replaceState(null, "", `#block-${blockId}`);
      target.dataset.flashing = "true";
      target.addEventListener(
        "animationend",
        () => delete target.dataset.flashing,
        { once: true },
      );
    });
  }

  useEffect(() => setMounted(true), []);

  useEffect(() => {
    window.addEventListener(OPEN_CREATOR_MENU, show);
    return () => window.removeEventListener(OPEN_CREATOR_MENU, show);
  }, [show]);

  if (!canOpenMenu) return null;

  return (
    <>
      <button
        type="button"
        className={styles.launch}
        aria-expanded={open}
        onClick={show}
      >
        <Settings2 size={17} aria-hidden="true" />
        <span>Creator menu</span>
        {missing > 0 ? (
          <span className={styles.count}>
            {missing}
            <span className="sr-only"> incomplete</span>
          </span>
        ) : null}
      </button>
      {mounted
        ? createPortal(
            <dialog
              ref={dialog}
              className={styles.dialog}
              aria-labelledby="creator-menu-title"
              onClose={() => setOpen(false)}
              onCancel={(event) => {
                event.preventDefault();
                close();
              }}
            >
              <header className={styles.header}>
                <div>
                  <h2 id="creator-menu-title">Creator menu</h2>
                  <p>Page details, publication and access.</p>
                </div>
                <button type="button" className={styles.close} onClick={close}>
                  <X size={18} aria-hidden="true" />
                  <span className="sr-only">Close creator menu</span>
                </button>
              </header>
              <div className={styles.body}>
                {props.isOwner && props.isDraft && props.readiness ? (
                  <PublishPanel
                    assetId={props.assetId}
                    kind={props.kind}
                    readiness={props.readiness}
                    onNavigateToBlock={goToBlock}
                  />
                ) : null}

                {props.isOwner && !props.isDraft && props.readiness ? (
                  <ShortfallPanel
                    kind={props.kind}
                    readiness={props.readiness}
                    onNavigateToBlock={goToBlock}
                  />
                ) : null}

                {props.isOwner ? (
                  <IdentityPanel
                    assetId={props.assetId}
                    initialName={props.name}
                    initialIsNsfw={props.isNsfw}
                    isDraft={props.isDraft}
                  />
                ) : null}

                {props.isOwner && !props.isDraft ? (
                  <DiscoveryControl
                    assetId={props.assetId}
                    creator={props.creator}
                    initialDiscovery={props.discovery}
                    frozen={props.withheld}
                  />
                ) : null}

                {props.isOwner && props.hasOriginal ? (
                  <PreservedPanel assetId={props.assetId} />
                ) : null}

                {props.isOwner && props.sealedBlocks ? (
                  <SealedPanel
                    assetId={props.assetId}
                    count={props.sealedBlocks}
                  />
                ) : null}

                {props.isOwner ? (
                  <DeleteControl
                    assetId={props.assetId}
                    creator={props.creator}
                    kind={props.kind}
                    isDraft={props.isDraft}
                    frozen={props.withheld}
                  />
                ) : null}

                {canWithhold ? (
                  <WithholdControl assetId={props.assetId} />
                ) : null}
              </div>
            </dialog>,
            document.body,
          )
        : null}
    </>
  );
}
