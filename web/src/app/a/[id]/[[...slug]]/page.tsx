import { ArrowLeft } from "lucide-react";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import Image, { type StaticImageData } from "next/image";
import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cache } from "react";
import archedCityWindow from "@/assets/art/arched-city-window.webp";
import armillarySphere from "@/assets/art/armillary-sphere.webp";
import booksAndQuill from "@/assets/art/books-and-quill.webp";
import compassStarRibbon from "@/assets/art/compass-star-ribbon.webp";
import floralCornerSpray from "@/assets/art/floral-corner-spray.webp";
import floralLantern from "@/assets/art/floral-lantern.webp";
import openStorybook from "@/assets/art/open-storybook.webp";
import sealedCompassLetter from "@/assets/art/sealed-compass-letter.webp";
import { Shell } from "@/components/layout/Shell";
import { FormattingNotice, RichText } from "@/components/ui/RichText";
import { type AssetDetail, fetchAsset } from "@/lib/api/query";
import { assetMetadata } from "@/lib/asset-metadata";
import { assetDisplayName } from "@/lib/asset-name";
import { assetRedirect } from "@/lib/asset-url";
import { KIND_LABELS } from "@/lib/kinds";
import { formattingWasRemoved } from "@/lib/rich-text";
import { AssetBlocks } from "./AssetBlocks";
import { AssetMedia } from "./AssetMedia";
import { DeleteControl } from "./DeleteControl";
import { DiscoveryControl } from "./DiscoveryControl";
import { DownloadPanel } from "./DownloadPanel";
import { IdentityPanel } from "./IdentityPanel";
import { PreservedPanel } from "./PreservedPanel";
import { PublishPanel } from "./PublishPanel";
import styles from "./page.module.css";
import { WithholdControl } from "./WithholdControl";
import { WithholdNotice } from "./WithholdNotice";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const DETAIL_ART: Record<AssetDetail["kind"], readonly StaticImageData[]> = {
  character: [floralLantern, floralCornerSpray, booksAndQuill],
  lorebook: [archedCityWindow, openStorybook, booksAndQuill],
  preset: [armillarySphere, compassStarRibbon],
  theme: [sealedCompassLetter, floralCornerSpray, floralLantern],
};

function detailArtworkFor(
  kind: AssetDetail["kind"],
  assetId: string,
): StaticImageData {
  const choices = DETAIL_ART[kind];
  const variant = Number.parseInt(assetId.slice(-2), 16) % choices.length;
  return choices[variant] ?? choices[0];
}

const loadAsset = cache(async (id: string): Promise<AssetDetail | null> => {
  if (!UUID.test(id)) return null;
  const cookie = (await cookies()).toString();
  return fetchAsset(id, cookie);
});

function ratingLabel(isNsfw: boolean | null): string {
  if (isNsfw === null) return "Rating not set";
  return isNsfw ? "Adult content" : "No adult content";
}

function browseTagHref(value: string): string {
  const quoted = value.includes(" ") ? `"${value}"` : value;
  return `/browse?q=${encodeURIComponent(`tag:${quoted}`)}`;
}

const TAG_PREVIEW_LIMIT = 8;

function TagLinks({ tags }: { tags: AssetDetail["tags"] }) {
  return (
    <ul className={styles.tagGrid}>
      {tags.map((tag) => (
        <li key={tag.value}>
          <Link href={browseTagHref(tag.value)}>{tag.label}</Link>
        </li>
      ))}
    </ul>
  );
}

function TagShelf({ tags }: { tags: AssetDetail["tags"] }) {
  const remaining = tags.slice(TAG_PREVIEW_LIMIT);

  return (
    <div className={styles.tagShelf}>
      <TagLinks tags={tags.slice(0, TAG_PREVIEW_LIMIT)} />
      {remaining.length > 0 ? (
        <details className={styles.moreTags}>
          <summary>
            <span className={styles.showTags}>Show all {tags.length} tags</span>
            <span className={styles.hideTags}>Show fewer tags</span>
          </summary>
          <TagLinks tags={remaining} />
        </details>
      ) : null}
    </div>
  );
}

export async function generateMetadata({
  params,
}: PageProps<"/a/[id]/[[...slug]]">): Promise<Metadata> {
  const asset = await loadAsset((await params).id);
  return asset ? assetMetadata(asset) : { title: "Not found" };
}

export default async function AssetPage({
  params,
}: PageProps<"/a/[id]/[[...slug]]">) {
  const { id, slug } = await params;
  const asset = await loadAsset(id);
  if (!asset) notFound();

  const canonical = assetRedirect({ id, slug }, asset);
  if (canonical) redirect(canonical);

  const kind = KIND_LABELS[asset.kind];
  const isDraft = asset.lifecycle === "draft";
  const hasCreatorArtwork = asset.media.length > 0;
  const detailArtwork = detailArtworkFor(asset.kind, asset.id);
  const made = new Date(asset.createdAt).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });

  return (
    <div className={styles.page}>
      <article>
        <section className={styles.hero}>
          {hasCreatorArtwork ? null : (
            <Image
              src={detailArtwork}
              alt=""
              sizes="(max-width: 680px) 320px, 42vw"
              className={styles.detailArtwork}
            />
          )}
          <Shell className={styles.heroShell}>
            <Link href="/browse" className={styles.back}>
              <ArrowLeft size={15} aria-hidden="true" />
              Back to the collection
            </Link>

            <div className={styles.heroLayout}>
              <AssetMedia
                id={asset.id}
                media={asset.media}
                kind={asset.kind}
                name={asset.name}
                isNsfw={asset.isNsfw}
                visibility={asset.visibility}
              />

              <div className={styles.identityLead}>
                <div className={styles.classification}>
                  <span className={styles.kind}>{kind}</span>
                  <span className={styles.rating}>
                    {ratingLabel(asset.isNsfw)}
                  </span>
                </div>
                <h1 className={asset.name ? undefined : styles.unnamed}>
                  {assetDisplayName(asset.name)}
                </h1>
                <p className={styles.byline}>
                  <span>Created by</span>
                  <Link className={styles.creator} href={`/@${asset.creator}`}>
                    {asset.creator}
                  </Link>
                  <span className={styles.sharedDate}>
                    {isDraft ? `Started ${made}` : `Shared ${made}`}
                  </span>
                </p>
              </div>

              <div className={styles.headerActions}>
                {isDraft && asset.isOwner && asset.readiness ? (
                  <PublishPanel
                    assetId={asset.id}
                    kind={kind.toLowerCase()}
                    readiness={asset.readiness}
                  />
                ) : null}

                <DownloadPanel
                  assetId={asset.id}
                  downloads={asset.downloads}
                  original={asset.original}
                  images={asset.media}
                />
              </div>

              <div className={styles.identityDetails}>
                {asset.blurb ? (
                  <div className={styles.blurb}>
                    <RichText text={asset.blurb} />
                    {formattingWasRemoved([asset.blurb]) ? (
                      <FormattingNotice />
                    ) : null}
                  </div>
                ) : (
                  <p className={styles.noBlurb}>
                    The creator has not written a blurb for this{" "}
                    {kind.toLowerCase()} yet.
                  </p>
                )}

                {asset.tags.length > 0 ? <TagShelf tags={asset.tags} /> : null}

                {asset.withhold ? (
                  <WithholdNotice withhold={asset.withhold} />
                ) : null}
              </div>
            </div>
          </Shell>
        </section>

        <Shell className={styles.contentShell}>
          <div className={styles.contentLayout}>
            <section className={styles.blocks} aria-label="Asset content">
              <AssetBlocks
                assetId={asset.id}
                blocks={asset.blocks}
                images={asset.media}
                addableSections={asset.addableSections ?? []}
                isOwner={asset.isOwner}
              />
            </section>

            <aside
              className={styles.rail}
              aria-label="Asset details and actions"
            >
              {asset.isOwner ? (
                <IdentityPanel
                  assetId={asset.id}
                  initialName={asset.name}
                  initialIsNsfw={asset.isNsfw}
                  isDraft={isDraft}
                />
              ) : null}

              {asset.isOwner ? (
                <section className={styles.creatorTools}>
                  <h2>Creator tools</h2>
                  {/* Discovery applies to a published asset only. */}
                  {isDraft ? null : (
                    <DiscoveryControl
                      assetId={asset.id}
                      creator={asset.creator}
                      initialDiscovery={asset.discovery}
                      frozen={Boolean(asset.withhold)}
                    />
                  )}
                  <DeleteControl
                    assetId={asset.id}
                    creator={asset.creator}
                    kind={kind.toLowerCase()}
                    isDraft={isDraft}
                    frozen={Boolean(asset.withhold)}
                  />
                  {asset.original ? (
                    <PreservedPanel assetId={asset.id} />
                  ) : null}
                </section>
              ) : null}

              {!isDraft && !asset.withhold ? (
                <WithholdControl assetId={asset.id} />
              ) : null}
            </aside>
          </div>
        </Shell>
      </article>
    </div>
  );
}
