"use client";

import Image from "next/image";
import { useState } from "react";
import type { BrowseAsset } from "@/lib/api/query";
import { DEFAULT_COVERS } from "@/lib/kinds";
import styles from "./FeaturedShelf.module.css";

export function ShelfPreview({ asset }: { asset: BrowseAsset }) {
  const [failed, setFailed] = useState(false);
  const creatorCover = failed ? null : asset.cover;
  const usesDefault = !creatorCover;
  const src = creatorCover?.url ?? DEFAULT_COVERS[asset.kind];

  return (
    <div className={styles.preview}>
      <Image
        src={src}
        alt=""
        fill
        sizes="76px"
        className={styles.previewImage}
        onError={usesDefault ? undefined : () => setFailed(true)}
        unoptimized={!usesDefault}
      />
    </div>
  );
}
