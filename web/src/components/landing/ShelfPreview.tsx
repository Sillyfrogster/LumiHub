"use client";

import Image from "next/image";
import { useState } from "react";
import { EmptyArtwork } from "@/components/media/EmptyArtwork";
import type { BrowseAsset } from "@/lib/api/query";
import styles from "./FeaturedShelf.module.css";

export function ShelfPreview({ asset }: { asset: BrowseAsset }) {
  const [failed, setFailed] = useState(false);
  const src = failed ? undefined : asset.cover?.url;

  return (
    <div className={styles.preview}>
      {src ? (
        <Image
          src={src}
          alt=""
          fill
          sizes="76px"
          className={styles.previewImage}
          onError={() => setFailed(true)}
          unoptimized
        />
      ) : (
        <EmptyArtwork kind={asset.kind} compact />
      )}
    </div>
  );
}
