"use client";

import { Eye, EyeOff } from "lucide-react";
import Image from "next/image";
import { useState } from "react";
import cornerBottom from "@/assets/art/corner-br.webp";
import cornerTop from "@/assets/art/corner-tl.webp";
import type { AssetImage, BrowseKind, NsfwVisibility } from "@/lib/api/query";
import { DEFAULT_COVERS } from "@/lib/kinds";
import styles from "./AssetMedia.module.css";

interface AssetMediaProps {
  media: AssetImage[];
  kind: BrowseKind;
  name: string;
  isNsfw: boolean;
  visibility: NsfwVisibility;
}

export function AssetMedia({
  media,
  kind,
  name,
  isNsfw,
  visibility,
}: AssetMediaProps) {
  const [chosen, setChosen] = useState(0);
  const [failed, setFailed] = useState(false);
  const shown = media[chosen];
  const useFallback = failed || !shown;

  return (
    <div className={styles.media}>
      <div className={styles.frame}>
        <Image
          className={styles.ornamentTop}
          src={cornerTop}
          alt=""
          aria-hidden="true"
          sizes="260px"
        />
        <Image
          className={styles.ornamentBottom}
          src={cornerBottom}
          alt=""
          aria-hidden="true"
          sizes="250px"
        />
        {useFallback ? (
          <Image
            className={styles.defaultCover}
            src={DEFAULT_COVERS[kind]}
            alt=""
            width={1024}
            height={1024}
            priority
          />
        ) : (
          <Image
            className={styles.cover}
            src={shown.detailUrl}
            alt={name}
            width={shown.width}
            height={shown.height}
            sizes="(max-width: 900px) 100vw, 420px"
            onError={() => setFailed(true)}
            priority
            unoptimized
          />
        )}
        {isNsfw ? (
          <p className={styles.flag}>
            {visibility === "shown" ? (
              <Eye size={13} aria-hidden="true" />
            ) : (
              <EyeOff size={13} aria-hidden="true" />
            )}
            {visibility === "shown" ? "Adult" : "Adult · blurred"}
          </p>
        ) : null}
      </div>

      {media.length > 1 ? (
        <ul className={styles.strip}>
          {media.map((image, index) => (
            <li key={image.id}>
              <button
                type="button"
                className={index === chosen ? styles.pickedThumb : styles.thumb}
                aria-current={index === chosen}
                aria-label={`Image ${index + 1} of ${media.length}`}
                onClick={() => {
                  setChosen(index);
                  setFailed(false);
                }}
              >
                <Image
                  src={image.thumbUrl}
                  alt=""
                  width={image.width}
                  height={image.height}
                  sizes="80px"
                  unoptimized
                />
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
