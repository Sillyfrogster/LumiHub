import { ArrowLeft, Download } from "lucide-react";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cache } from "react";
import { Shell } from "@/components/layout/Shell";
import { type AssetDetail, fetchAsset } from "@/lib/api/query";
import { assetMetadata } from "@/lib/asset-metadata";
import { assetDisplayName } from "@/lib/asset-name";
import { assetRedirect } from "@/lib/asset-url";
import { KIND_LABELS } from "@/lib/kinds";
import { AssetBlocks } from "./AssetBlocks";
import { AssetMedia } from "./AssetMedia";
import { DeleteControl } from "./DeleteControl";
import { DiscoveryControl } from "./DiscoveryControl";
import { IdentityPanel } from "./IdentityPanel";
import { PublishPanel } from "./PublishPanel";
import styles from "./page.module.css";
import { WithholdControl } from "./WithholdControl";
import { WithholdNotice } from "./WithholdNotice";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * One read per request, shared by the page and its metadata. Resolution is by
 * id alone, so a wrong slug never reaches the API.
 */
const loadAsset = cache(async (id: string): Promise<AssetDetail | null> => {
  if (!UUID.test(id)) return null;
  const cookie = (await cookies()).toString();
  return fetchAsset(id, cookie);
});

/**
 * The adult content answer as the header states it. A draft that has not been
 * asked keeps a third state, so nothing reads as a no by default.
 */
function ratingLabel(isNsfw: boolean | null): string {
  if (isNsfw === null) return "Rating not set";
  return isNsfw ? "Adult content" : "No adult content";
}

function browseTagHref(value: string): string {
  const quoted = value.includes(" ") ? `"${value}"` : value;
  return `/browse?q=${encodeURIComponent(`tag:${quoted}`)}`;
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
  const made = new Date(asset.createdAt).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });

  return (
    <div className={styles.page}>
      <article>
        <section className={styles.hero}>
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

              <div className={styles.identity}>
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
                  Created by
                  <Link className={styles.creator} href={`/@${asset.creator}`}>
                    {asset.creator}
                  </Link>
                </p>

                {asset.blurb ? (
                  <p className={styles.blurb}>{asset.blurb}</p>
                ) : (
                  <p className={styles.noBlurb}>
                    The creator has not written a blurb for this{" "}
                    {kind.toLowerCase()} yet.
                  </p>
                )}

                {asset.tags.length > 0 ? (
                  <ul className={styles.tags}>
                    {asset.tags.map((tag) => (
                      <li key={tag.value}>
                        <Link href={browseTagHref(tag.value)}>{tag.label}</Link>
                      </li>
                    ))}
                  </ul>
                ) : null}

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
              {isDraft ? (
                asset.isOwner && asset.readiness ? (
                  <PublishPanel
                    assetId={asset.id}
                    kind={kind.toLowerCase()}
                    readiness={asset.readiness}
                  />
                ) : null
              ) : asset.hasSourceFile ? (
                <section className={styles.downloadPanel}>
                  <h2>Keep the original</h2>
                  <p>Download the creator’s source file as it was shared.</p>
                  <a href={`/download/${asset.id}`}>
                    <Download size={16} aria-hidden="true" />
                    Download source file
                  </a>
                </section>
              ) : null}

              {asset.isOwner ? (
                <IdentityPanel
                  assetId={asset.id}
                  initialName={asset.name}
                  initialIsNsfw={asset.isNsfw}
                  isDraft={isDraft}
                />
              ) : null}

              <section className={styles.facts}>
                <h2>About this {kind.toLowerCase()}</h2>
                <dl>
                  <div>
                    <dt>Creator</dt>
                    <dd>
                      <Link href={`/@${asset.creator}`}>{asset.creator}</Link>
                    </dd>
                  </div>
                  <div>
                    <dt>Kind</dt>
                    <dd>{kind}</dd>
                  </div>
                  <div>
                    <dt>Content</dt>
                    <dd>{ratingLabel(asset.isNsfw)}</dd>
                  </div>
                  <div>
                    <dt>Shared</dt>
                    <dd>{made}</dd>
                  </div>
                </dl>
              </section>

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
