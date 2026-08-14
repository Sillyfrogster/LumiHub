"use client";

import { Eye, EyeOff } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import type { BrowseAsset, NsfwVisibility } from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";
import styles from "./BrowseCard.module.css";

const KIND_LABELS: Record<BrowseAsset["kind"], string> = {
  character: "Character",
  lorebook: "Lorebook",
  preset: "Preset",
  theme: "Theme",
};

const DEFAULT_COVERS: Record<BrowseAsset["kind"], string> = {
  character: "/covers/cover-default-character.webp",
  lorebook: "/covers/cover-default-lorebook.webp",
  preset: "/covers/cover-default-preset.webp",
  theme: "/covers/cover-default-theme.webp",
};

export function BrowseCard({
  asset,
  visibility,
}: {
  asset: BrowseAsset;
  visibility: NsfwVisibility;
}) {
  const [failed, setFailed] = useState(false);
  const fallback = DEFAULT_COVERS[asset.kind];
  const src = failed || !asset.cover ? fallback : asset.cover.url;

  return (
    <li className={styles.item}>
      <Link href={assetHref(asset.id, asset.name)} className={styles.card}>
        <div className={styles.cover}>
          <Image
            src={src}
            alt=""
            fill
            sizes="(max-width: 560px) 86vw, (max-width: 900px) 42vw, (max-width: 1240px) 27vw, 260px"
            className={asset.cover && !failed ? styles.art : styles.defaultArt}
            onError={() => setFailed(true)}
            unoptimized
          />
          <span className={styles.kind}>{KIND_LABELS[asset.kind]}</span>
          {asset.isNsfw ? (
            <span className={styles.nsfw}>
              {visibility === "blurred" ? (
                <EyeOff size={12} aria-hidden="true" />
              ) : (
                <Eye size={12} aria-hidden="true" />
              )}
              {visibility === "blurred" ? "Adult · blurred" : "Adult"}
            </span>
          ) : null}
        </div>
        <div className={styles.identity}>
          <h3>{asset.name}</h3>
          <p>@{asset.creator}</p>
        </div>
      </Link>
    </li>
  );
}
