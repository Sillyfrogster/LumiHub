"use client";

import { CircleHelp, Eye, EyeOff } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import type { BrowseAsset, NsfwVisibility } from "@/lib/api/query";
import { assetDisplayName } from "@/lib/asset-name";
import { assetHref } from "@/lib/asset-url";
import { DEFAULT_COVERS, KIND_LABELS } from "@/lib/kinds";
import styles from "./BrowseCard.module.css";

export function BrowseCard({
  asset,
  visibility,
}: {
  asset: BrowseAsset;
  visibility: NsfwVisibility;
}) {
  const [failed, setFailed] = useState(false);
  const creatorCover = failed ? null : asset.cover;
  const usesDefault = !creatorCover;
  const src = creatorCover?.url ?? DEFAULT_COVERS[asset.kind];
  const withheldAt = asset.withhold
    ? new Date(asset.withhold.at).toLocaleString("en-GB", {
        dateStyle: "medium",
        timeStyle: "short",
      })
    : "";

  return (
    <li className={styles.item}>
      <Link href={assetHref(asset.id, asset.name)} className={styles.card}>
        <div className={styles.cover}>
          <Image
            src={src}
            alt=""
            fill
            sizes="(max-width: 560px) 86vw, (max-width: 900px) 42vw, (max-width: 1240px) 27vw, 260px"
            className={usesDefault ? styles.defaultArt : styles.art}
            onError={usesDefault ? undefined : () => setFailed(true)}
            unoptimized={!usesDefault}
          />
          <span className={styles.kind}>{KIND_LABELS[asset.kind]}</span>
          {asset.isNsfw === null ? (
            <span className={styles.unrated}>
              <CircleHelp size={12} aria-hidden="true" />
              Rating not set
            </span>
          ) : asset.isNsfw ? (
            <span className={styles.nsfw}>
              {visibility !== "shown" ? (
                <EyeOff size={12} aria-hidden="true" />
              ) : (
                <Eye size={12} aria-hidden="true" />
              )}
              {visibility !== "shown" ? "Adult · blurred" : "Adult"}
            </span>
          ) : null}
          {asset.ownerState ? (
            <span className={styles.ownerState}>{asset.ownerState}</span>
          ) : null}
        </div>
        <div className={styles.identity}>
          <h3>{assetDisplayName(asset.name)}</h3>
          <p>@{asset.creator}</p>
        </div>
      </Link>
      {asset.withhold ? (
        <div className={styles.withhold}>
          <strong>{asset.withhold.reason}</strong>
          <span>
            @{asset.withhold.actor} · {withheldAt}
          </span>
        </div>
      ) : null}
    </li>
  );
}
