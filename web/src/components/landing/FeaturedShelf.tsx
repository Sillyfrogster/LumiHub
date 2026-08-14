import Image from "next/image";
import Link from "next/link";
import shelfArt from "@/assets/art/full/home-shelf.webp";
import { Shell } from "@/components/layout/Shell";
import type { BrowseAsset, NsfwVisibility } from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";
import { KIND_LABELS } from "@/lib/kinds";
import styles from "./FeaturedShelf.module.css";
import { ShelfPreview } from "./ShelfPreview";

export function FeaturedShelf({
  assets,
  visibility,
  unavailable,
}: {
  assets: BrowseAsset[];
  visibility: NsfwVisibility;
  unavailable: boolean;
}) {
  return (
    <Shell as="section" className={styles.section}>
      <header className={styles.header}>
        <div>
          <h2>New on the shelf</h2>
          <p>The latest work, kept close to the form its creator gave it.</p>
        </div>
        <Link href="/browse">Browse the collection →</Link>
      </header>

      {assets.length ? (
        <div className={styles.stage}>
          <Image
            src={shelfArt}
            alt=""
            aria-hidden="true"
            className={styles.shelfArt}
            sizes="(max-width: 620px) 760px, 100vw"
          />
          <ul className={styles.grid}>
            {assets.map((asset) => (
              <li key={asset.id}>
                <Link
                  href={assetHref(asset.id, asset.name)}
                  className={styles.card}
                >
                  <ShelfPreview asset={asset} />
                  <span className={styles.identity}>
                    <small>
                      {KIND_LABELS[asset.kind]}
                      {asset.isNsfw
                        ? visibility === "shown"
                          ? " · Adult"
                          : " · Adult, blurred"
                        : ""}
                    </small>
                    <strong>{asset.name}</strong>
                    <span>@{asset.creator}</span>
                  </span>
                  <span className={styles.arrow} aria-hidden="true">
                    →
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <div className={styles.empty}>
          <p>
            {unavailable
              ? "The shelf could not be opened just now."
              : "The first additions are still being bound."}
          </p>
          <Link href="/browse">Open the full collection</Link>
        </div>
      )}
    </Shell>
  );
}
