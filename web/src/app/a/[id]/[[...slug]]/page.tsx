import { ArrowLeft, Download } from "lucide-react";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import Image from "next/image";
import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cache } from "react";
import sprig from "@/assets/art/sprig.webp";
import wash from "@/assets/art/wash.webp";
import { Shell } from "@/components/layout/Shell";
import { type AssetDetail, fetchAsset } from "@/lib/api/query";
import { assetMetadata } from "@/lib/asset-metadata";
import { assetRedirect } from "@/lib/asset-url";
import { KIND_LABELS } from "@/lib/kinds";
import { AssetMedia } from "./AssetMedia";
import { DiscoveryControl } from "./DiscoveryControl";
import styles from "./page.module.css";

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
  const made = new Date(asset.createdAt).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });

  return (
    <div className={styles.page}>
      <div className={styles.band}>
        <div className={styles.wash}>
          <Image src={wash} alt="" fill priority sizes="100vw" />
        </div>
        <Image className={styles.sprig} src={sprig} alt="" sizes="178px" />
      </div>

      <Shell as="article" className={styles.body}>
        <Link href="/browse" className={styles.back}>
          <ArrowLeft size={15} aria-hidden="true" />
          Back to the collection
        </Link>

        <div className={styles.layout}>
          <AssetMedia
            id={asset.id}
            media={asset.media}
            kind={asset.kind}
            name={asset.name}
            isNsfw={asset.isNsfw}
            visibility={asset.visibility}
          />

          <div className={styles.identity}>
            <p className={styles.kind}>{kind}</p>
            <h1>{asset.name}</h1>
            <p className={styles.byline}>
              by <strong>{asset.creator}</strong>
              <span className={styles.dot} aria-hidden="true" />
              <span className={styles.made}>{made}</span>
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

            <a className={styles.download} href={`/download/${asset.id}`}>
              <Download size={16} aria-hidden="true" />
              Download the file
            </a>

            <DiscoveryControl
              assetId={asset.id}
              creator={asset.creator}
              initialDiscovery={asset.discovery}
            />
          </div>
        </div>
      </Shell>
    </div>
  );
}
