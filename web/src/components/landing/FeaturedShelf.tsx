import Link from "next/link";
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
          <h2>Recently published</h2>
          <p>
            Creator work from across the catalog, shown without recutting it.
          </p>
        </div>
        <Link href="/browse">See the full catalog →</Link>
      </header>

      {assets.length ? (
        <div className={styles.stage}>
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
              ? "The catalog could not be loaded just now."
              : "Nothing has been published yet."}
          </p>
          <Link href="/browse">Open the catalog</Link>
        </div>
      )}
    </Shell>
  );
}
