"use client";

import { Eye, EyeOff, ImagePlus } from "lucide-react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { DefaultCover } from "@/components/media/DefaultCover";
import {
  type AssetImage,
  addAssetImage,
  type BrowseKind,
  type NsfwVisibility,
} from "@/lib/api/query";
import { useAuth } from "@/lib/auth";
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
  // Null while a draft has not been asked the adult content question.
  isNsfw: boolean | null;
  visibility: NsfwVisibility;
  isOwner: boolean;
}

export function AssetMedia({
  id,
  media,
  kind,
  name,
  isNsfw,
  visibility,
  isOwner,
}: AssetMediaProps) {
  const router = useRouter();
  const { account } = useAuth();
  const fileInput = useRef<HTMLInputElement>(null);
  const presentationMedia = media.filter((image) => image.role !== "pack_item");
  const [chosen, setChosen] = useState<number | null>(() => {
    const cover = presentationMedia.findIndex((image) => image.isCover);
    return cover >= 0 ? cover : null;
  });
  const [failed, setFailed] = useState(false);
  const [revealed, setRevealed] = useState(false);
  const [signedOutVisibility, setSignedOutVisibility] =
    useState<NsfwVisibility>();
  const [uploading, setUploading] = useState(false);
  const [uploadStatus, setUploadStatus] = useState<{
    message: string;
    failed: boolean;
  }>();
  const hasCover = presentationMedia.some((image) => image.isCover);
  const shown = chosen === null ? undefined : presentationMedia[chosen];
  const useFallback = failed || !shown;
  const showClear =
    visibility === "shown" || signedOutVisibility === "shown" || revealed;
  const canReveal = isNsfw === true && !showClear && !useFallback;

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

  async function replaceCover(file: File | null) {
    if (!file || uploading) return;
    setUploading(true);
    setUploadStatus(undefined);
    try {
      await addAssetImage(id, file, "avatar");
      setUploadStatus({ message: "Cover changed.", failed: false });
      router.refresh();
    } catch (error) {
      setUploadStatus({
        message:
          error instanceof Error
            ? error.message
            : "The cover could not be changed. Try again.",
        failed: true,
      });
    } finally {
      setUploading(false);
      if (fileInput.current) fileInput.current.value = "";
    }
  }

  return (
    <div className={styles.media}>
      <div className={styles.frame}>
        {useFallback ? (
          <div className={styles.defaultCover}>
            <DefaultCover kind={kind} />
          </div>
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
        {isNsfw === true ? (
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

      {presentationMedia.length > 1 ||
      (presentationMedia.length === 1 && chosen === null) ? (
        <ul className={styles.strip}>
          {presentationMedia.map((image, index) => (
            <li key={image.id}>
              <button
                type="button"
                className={index === chosen ? styles.pickedThumb : styles.thumb}
                aria-current={index === chosen}
                aria-label={`Image ${index + 1} of ${presentationMedia.length}`}
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

      {isOwner ? (
        <div className={styles.coverActions}>
          <label className={styles.coverUpload}>
            <ImagePlus size={16} aria-hidden="true" />
            {uploading
              ? "Uploading…"
              : hasCover
                ? "Replace cover"
                : "Add cover"}
            <input
              ref={fileInput}
              type="file"
              accept="image/png,image/jpeg,image/webp,image/gif"
              disabled={uploading}
              onChange={(event) =>
                void replaceCover(event.target.files?.[0] ?? null)
              }
            />
          </label>
          {uploadStatus ? (
            <p
              className={styles.uploadMessage}
              data-error={uploadStatus.failed || undefined}
              role={uploadStatus.failed ? "alert" : "status"}
            >
              {uploadStatus.message}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function clearVariant(url: string) {
  return url.replace("_blurred/", "/");
}
