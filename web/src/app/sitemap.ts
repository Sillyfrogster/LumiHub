import type { MetadataRoute } from "next";
import {
  type AssetListParams,
  type BrowsePage,
  fetchAssets,
} from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";

export const dynamic = "force-dynamic";

const siteUrl = process.env.SITE_URL ?? "http://localhost:8000";

type ListSitemapAssets = (
  params: AssetListParams,
) => Promise<Pick<BrowsePage, "items" | "nextCursor">>;

export async function buildSitemap(
  listAssets: ListSitemapAssets,
): Promise<MetadataRoute.Sitemap> {
  const entries: MetadataRoute.Sitemap = [
    { url: new URL("/", siteUrl).href },
    { url: new URL("/browse", siteUrl).href },
  ];
  let before: string | undefined;
  let beforeId: string | undefined;

  do {
    const page = await listAssets({
      limit: 24,
      nsfw: "shown",
      before,
      beforeId,
    });
    for (const asset of page.items) {
      entries.push({
        url: new URL(assetHref(asset.id, asset.name), siteUrl).href,
      });
    }
    before = page.nextCursor?.before;
    beforeId = page.nextCursor?.beforeId;
  } while (before && beforeId);

  return entries;
}

export default function sitemap(): Promise<MetadataRoute.Sitemap> {
  return buildSitemap(fetchAssets);
}
