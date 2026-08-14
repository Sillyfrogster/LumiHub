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
import { assetHref } from "@/lib/asset-url";
import { DEFAULT_COVERS, KIND_LABELS } from "@/lib/kinds";
import { AssetMedia } from "./AssetMedia";
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
  if (!asset) return { title: "Not found" };

  const title = `${asset.name} · ${KIND_LABELS[asset.kind]}`;
  const description = asset.blurb || `A ${asset.kind} by ${asset.creator}.`;
  const image = asset.preview ?? DEFAULT_COVERS[asset.kind];

  return {
    title,
    description,
    alternates: { canonical: assetHref(asset.id, asset.name) },
    robots: asset.discovery === "unlisted" ? { index: false } : undefined,
    openGraph: {
      type: "article",
      siteName: "LumiHub",
      title,
      description,
      url: assetHref(asset.id, asset.name),
      images: [{ url: image, alt: asset.name, width: 1200, height: 630 }],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: [image],
    },
  };
}

export default async function AssetPage({
  params,
}: PageProps<"/a/[id]/[[...slug]]">) {
  const { id, slug } = await params;
  const asset = await loadAsset(id);
  if (!asset) notFound();

  const here = slug?.length ? `/a/${id}/${slug.join("/")}` : `/a/${id}`;
  const canonical = assetHref(asset.id, asset.name);
  if (here !== canonical) redirect(canonical);

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
        <Image
          className={styles.sprig}
          src={sprig}
          alt=""
          sizes="226px"
          priority
        />
      </div>

      <Shell as="article" className={styles.body}>
        <Link href="/browse" className={styles.back}>
          <ArrowLeft size={15} aria-hidden="true" />
          Back to the collection
        </Link>

        <div className={styles.layout}>
          <AssetMedia
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
          </div>
        </div>
      </Shell>
    </div>
  );
}
