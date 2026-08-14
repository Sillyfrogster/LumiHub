import type { Metadata } from "next";
import type { AssetDetail } from "./api/query";
import { assetHref } from "./asset-url";
import { KIND_LABELS } from "./kinds";

/**
 * The tags a chat window reads when somebody pastes an asset's address. An
 * unlisted asset asks not to be indexed, which reduces discovery and is not a
 * boundary, so it still invites a crawler to follow its links.
 */
export function assetMetadata(asset: AssetDetail): Metadata {
  const title = `${asset.name} · ${KIND_LABELS[asset.kind]}`;
  const description = asset.blurb || `A ${asset.kind} by ${asset.creator}.`;
  const url = assetHref(asset.id, asset.name);
  const images = asset.preview
    ? [{ url: asset.preview, alt: asset.name, width: 1200, height: 630 }]
    : undefined;

  return {
    title,
    description,
    alternates: { canonical: url },
    robots: asset.discovery === "unlisted" ? { index: false } : undefined,
    openGraph: {
      type: "article",
      siteName: "LumiHub",
      title,
      description,
      url,
      images,
    },
    twitter: {
      card: asset.preview ? "summary_large_image" : "summary",
      title,
      description,
      images: asset.preview ? [asset.preview] : undefined,
    },
  };
}
