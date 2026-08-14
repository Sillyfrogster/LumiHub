"use client";

import { Eye, EyeOff } from "lucide-react";
import Image from "next/image";
import { useEffect, useState } from "react";
import type { AssetImage, BrowseKind, NsfwVisibility } from "@/lib/api/query";
import { useAuth } from "@/lib/auth";
import { DEFAULT_COVERS } from "@/lib/kinds";
import {
  readAssetReveal,
  readSessionVisibility,
  writeAssetReveal,
} from "@/lib/nsfw-visibility";
import styles from "./AssetMedia.module.css";

interface AssetMediaProps {
  id: string;
  media: AssetImage[];
  kind: BrowseKind;
  name: string;
  isNsfw: boolean;
  visibility: NsfwVisibility;
}

export function AssetMedia({
  id,
  media,
  kind,
  name,
  isNsfw,
  visibility,
}: AssetMediaProps) {
  const { account } = useAuth();
  const [chosen, setChosen] = useState(0);
  const [failed, setFailed] = useState(false);
  const [revealed, setRevealed] = useState(false);
  const [signedOutVisibility, setSignedOutVisibility] =
    useState<NsfwVisibility>();
  const shown = media[chosen];
  const useFallback = failed || !shown;
  const showClear =
    visibility === "shown" || signedOutVisibility === "shown" || revealed;
  const canReveal = isNsfw && !showClear && !useFallback;

  useEffect(() => {
    if (account === undefined) return;
    if (account) {
      setSignedOutVisibility(undefined);
      return;
    }
    setSignedOutVisibility(readSessionVisibility());
  }, [account]);

  useEffect(() => {
    if (!canReveal) return;
    setRevealed(readAssetReveal(id));
  }, [canReveal, id]);

  function reveal() {
    setRevealed(true);
    writeAssetReveal(id);
  }

  return (
    <div className={styles.media}>
      <div className={styles.frame}>
        {useFallback ? (
          <Image
            className={styles.defaultCover}
            src={DEFAULT_COVERS[kind]}
            alt=""
            width={1086}
            height={1448}
            priority
          />
        ) : (
          <Image
            className={styles.cover}
            src={showClear ? clearVariant(shown.detailUrl) : shown.detailUrl}
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
            {showClear ? (
              <Eye size={13} aria-hidden="true" />
            ) : (
              <EyeOff size={13} aria-hidden="true" />
            )}
            {showClear ? "Adult" : "Adult · blurred"}
          </p>
        ) : null}
        {canReveal && !revealed ? (
          <button type="button" className={styles.reveal} onClick={reveal}>
            <Eye size={16} aria-hidden="true" />
            Show images
          </button>
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
                  src={
                    showClear ? clearVariant(image.thumbUrl) : image.thumbUrl
                  }
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

function clearVariant(url: string) {
  return url.replace("_blurred/", "/");
}
