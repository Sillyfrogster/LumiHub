"use client";

import Image from "next/image";
import { useState } from "react";
import { DefaultCover } from "@/components/media/DefaultCover";
import type { BrowseAsset } from "@/lib/api/query";
import styles from "./FeaturedShelf.module.css";

export function ShelfPreview({ asset }: { asset: BrowseAsset }) {
  const [failed, setFailed] = useState(false);
  const creatorCover = failed ? null : asset.cover;

  return (
    <div className={styles.preview}>
      {creatorCover ? (
        <Image
          src={creatorCover.url}
          alt=""
          fill
          sizes="76px"
          className={styles.previewImage}
          onError={() => setFailed(true)}
          unoptimized
        />
      ) : (
        <DefaultCover kind={asset.kind} compact />
      )}
    </div>
  );
}
