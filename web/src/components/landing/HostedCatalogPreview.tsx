"use client";

import Image from "next/image";
import { useState } from "react";
import { DefaultCover } from "@/components/media/DefaultCover";
import type { BrowseAsset } from "@/lib/api/query";
import styles from "./HostedLanding.module.css";

export function HostedCatalogPreview({ asset }: { asset: BrowseAsset }) {
  const [failed, setFailed] = useState(false);
  const cover = failed ? null : asset.cover;

  return (
    <span className={styles.assetMedia} aria-hidden="true">
      {cover ? (
        <Image
          src={cover.url}
          alt=""
          fill
          sizes="(max-width: 720px) 72px, 84px"
          className={styles.assetImage}
          onError={() => setFailed(true)}
          unoptimized
        />
      ) : (
        <DefaultCover kind={asset.kind} compact />
      )}
    </span>
  );
}
