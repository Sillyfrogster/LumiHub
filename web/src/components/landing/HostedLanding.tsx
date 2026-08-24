import { ArrowRight, Cable, Download, FileCheck2 } from "lucide-react";
import Link from "next/link";
import { Shell } from "@/components/layout/Shell";
import { ScrollReveal } from "@/components/motion/ScrollReveal";
import { Button } from "@/components/ui/Button";
import type { BrowseAsset, BrowsePage, NsfwVisibility } from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";
import { KIND_LABELS } from "@/lib/kinds";
import { HostedCatalogPreview } from "./HostedCatalogPreview";
import styles from "./HostedLanding.module.css";

const HERO_PROMISES = [
  { label: "Source file intact", Icon: FileCheck2 },
  { label: "Compatible exports", Icon: Download },
  { label: "Linked delivery", Icon: Cable },
] as const;

type HostedLandingProps = {
  assets: BrowseAsset[];
  visibility: NsfwVisibility;
  suppressed: number;
  emptyState: BrowsePage["emptyState"];
  unavailable: boolean;
};

export function HostedLanding({
  assets,
  visibility,
  suppressed,
  emptyState,
  unavailable,
}: HostedLandingProps) {
  return (
    <div className={styles.page}>
      <ScrollReveal />

      <section className={styles.hero}>
        <div className={styles.environment} aria-hidden="true" />

        <Shell className={styles.heroShell}>
          <div className={styles.heroCopy}>
            <h1>AI roleplay, in one catalog.</h1>
            <p>
              Characters, lorebooks, and presets—one catalog, independent of any
              single application&apos;s format.
            </p>
            <div className={styles.actions}>
              <Button href="/browse" size="large">
                Browse the catalog
                <ArrowRight size={16} strokeWidth={1.5} aria-hidden="true" />
              </Button>
              <Button href="/upload" variant="outline" size="large">
                Publish
              </Button>
            </div>

            <ul
              className={styles.heroPromises}
              aria-label="How Illarin works across applications"
            >
              {HERO_PROMISES.map(({ label, Icon }) => (
                <li key={label}>
                  <Icon size={16} strokeWidth={1.35} aria-hidden="true" />
                  <span>{label}</span>
                </li>
              ))}
            </ul>
          </div>

          <div className={styles.mobileArt} aria-hidden="true" />

          <section
            className={styles.catalogSeam}
            aria-labelledby="landing-catalog-title"
          >
            <header className={styles.catalogHeader}>
              <div>
                <h2 id="landing-catalog-title">Recently published</h2>
                <p>Real creator work, entering the catalog as itself.</p>
              </div>
              <Link href="/browse">
                See the full catalog
                <ArrowRight size={15} strokeWidth={1.5} aria-hidden="true" />
              </Link>
            </header>

            {assets.length > 0 ? (
              <ul className={styles.catalogGrid}>
                {assets.map((asset) => (
                  <li key={asset.id}>
                    <Link
                      href={assetHref(asset.id, asset.name)}
                      className={styles.assetLink}
                    >
                      <HostedCatalogPreview asset={asset} />
                      <span className={styles.assetIdentity}>
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
                      <ArrowRight
                        className={styles.assetArrow}
                        size={15}
                        strokeWidth={1.35}
                        aria-hidden="true"
                      />
                    </Link>
                  </li>
                ))}
              </ul>
            ) : (
              <CatalogMessage
                unavailable={unavailable}
                emptyState={emptyState}
                suppressed={suppressed}
              />
            )}

            {assets.length > 0 && suppressed > 0 ? (
              <p className={styles.suppressedNote}>
                {suppressed} more {suppressed === 1 ? "item is" : "items are"}{" "}
                outside your content preference.
              </p>
            ) : null}
          </section>
        </Shell>
      </section>

      <section className={styles.journey} data-reveal>
        <Shell className={styles.journeyShell}>
          <div className={styles.compositionCopy}>
            <h2>Edit the page itself.</h2>
            <p>
              Import a file or start from scratch. Choose the layout, edit the
              content, and publish from the same page readers will open.
            </p>
          </div>

          <figure
            className={styles.builderProof}
            role="img"
            aria-label="The character block editor with block naming, page layout, width, and description controls"
            data-reveal
          />

          <div className={styles.deliveryPlane} data-reveal>
            <div className={styles.deliveryCopy}>
              <h2>Download it. Or send it.</h2>
              <p>
                Each asset offers only the formats its content can carry.
                Download a file, or send it to a linked instance when one is
                connected.
              </p>
              <Link href="/browse" className={styles.deliveryAction}>
                Find something to use
                <ArrowRight size={16} strokeWidth={1.5} aria-hidden="true" />
              </Link>
            </div>
            <ul className={styles.deliveryOutcomes}>
              <li>
                <Download size={22} strokeWidth={1.35} aria-hidden="true" />
                <span>
                  <strong>Download</strong>
                  <small>The baseline</small>
                </span>
              </li>
              <li>
                <Cable size={22} strokeWidth={1.35} aria-hidden="true" />
                <span>
                  <strong>Linked delivery</strong>
                  <small>Optional</small>
                </span>
              </li>
            </ul>
          </div>
        </Shell>
      </section>
    </div>
  );
}

function CatalogMessage({
  unavailable,
  emptyState,
  suppressed,
}: Pick<HostedLandingProps, "unavailable" | "emptyState" | "suppressed">) {
  let message = "The catalog is waiting for its first published work.";

  if (unavailable) {
    message = "The catalog could not be reached just now.";
  } else if (emptyState === "suppressed" || suppressed > 0) {
    message = "Published work is outside your current content preference.";
  }

  return (
    <div className={styles.catalogMessage}>
      <p>{message}</p>
      <Link href="/browse">Open the catalog</Link>
    </div>
  );
}
