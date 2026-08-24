import { ArrowLeft } from "lucide-react";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cache } from "react";
import { Shell } from "@/components/layout/Shell";
import { ChipSet } from "@/components/ui/Chip";
import { FormattingNotice, RichText } from "@/components/ui/RichText";
import { type AssetDetail, fetchAsset } from "@/lib/api/query";
import { assetMetadata } from "@/lib/asset-metadata";
import { assetDisplayName } from "@/lib/asset-name";
import { assetHoldsNothing } from "@/lib/asset-page-content";
import { assetRedirect, isAssetId } from "@/lib/asset-url";
import { KIND_LABELS } from "@/lib/kinds";
import { formattingWasRemoved } from "@/lib/rich-text";
import { AssetBlocks } from "./AssetBlocks";
import { AssetMedia } from "./AssetMedia";
import { DownloadPanel } from "./DownloadPanel";
import { DraftHeaderActions } from "./DraftHeaderActions";
import styles from "./page.module.css";
import { SendToInstance } from "./SendToInstance";
import { WithholdNotice } from "./WithholdNotice";

const loadAsset = cache(async (id: string): Promise<AssetDetail | null> => {
  if (!isAssetId(id)) return null;
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

function TagShelf({ tags }: { tags: AssetDetail["tags"] }) {
  return (
    <ChipSet
      className={styles.tagShelf}
      limit={TAG_PREVIEW_LIMIT}
      items={tags.map((tag) => ({
        id: tag.value,
        label: tag.label,
        href: browseTagHref(tag.value),
      }))}
    />
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
  const formattedCreatedDate = new Date(asset.createdAt).toLocaleDateString(
    "en-GB",
    {
      day: "numeric",
      month: "long",
      year: "numeric",
    },
  );
  const holdsNothing = assetHoldsNothing(asset.blocks);
  const hasHeaderActions = Boolean(
    (isDraft && asset.isOwner && asset.readiness) ||
      asset.downloads.length > 0 ||
      asset.original,
  );

  return (
    <div className={styles.page}>
      <article>
        <section className={styles.hero}>
          <Shell className={styles.heroShell}>
            <Link href="/browse" className={styles.back}>
              <ArrowLeft size={15} aria-hidden="true" />
              Back to the collection
            </Link>

            <div
              className={`${styles.heroLayout} ${
                hasHeaderActions ? "" : styles.heroLayoutWithoutActions
              }`}
            >
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
                    {isDraft
                      ? `Started ${formattedCreatedDate}`
                      : `Shared ${formattedCreatedDate}`}
                  </span>
                </p>
              </div>

              {hasHeaderActions ? (
                <div className={styles.headerActions}>
                  {isDraft && asset.isOwner && asset.readiness ? (
                    <DraftHeaderActions />
                  ) : null}

                  <DownloadPanel
                    assetId={asset.id}
                    downloads={asset.downloads}
                    original={asset.original}
                    images={asset.media}
                    holdsNothing={holdsNothing}
                  />

                  {isDraft ? null : <SendToInstance assetId={asset.id} />}
                </div>
              ) : null}

              <div className={styles.assetMediaSlot}>
                <AssetMedia
                  id={asset.id}
                  media={asset.media}
                  kind={asset.kind}
                  name={asset.name}
                  isNsfw={asset.isNsfw}
                  visibility={asset.visibility}
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
          <section className={styles.blocks} aria-label="Asset content">
            <AssetBlocks
              assetId={asset.id}
              kind={asset.kind}
              blocks={asset.blocks}
              images={asset.media}
              addableBlocks={asset.addableBlocks ?? []}
              isOwner={asset.isOwner}
              creatorMenu={{
                assetId: asset.id,
                creator: asset.creator,
                kind: kind.toLowerCase(),
                name: asset.name,
                isNsfw: asset.isNsfw,
                isDraft,
                isOwner: asset.isOwner,
                discovery: asset.discovery,
                withheld: Boolean(asset.withhold),
                hasOriginal: Boolean(asset.original),
                readiness: asset.readiness,
                sealedBlocks: asset.sealedBlocks,
              }}
            />
          </section>
        </Shell>
      </article>
    </div>
  );
}
