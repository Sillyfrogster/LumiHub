import type { MetadataRoute } from "next";
import {
  type AssetListParams,
  type BrowsePage,
  fetchAssets,
} from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";
import { siteUrl } from "@/lib/site-metadata";

export const dynamic = "force-dynamic";

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
  let cursor: BrowsePage["nextCursor"];

  do {
    const page = await listAssets({
      limit: 24,
      nsfw: "shown",
      before: cursor?.before,
      beforeId: cursor?.beforeId,
    });
    for (const asset of page.items) {
      entries.push({
        url: new URL(assetHref(asset.id, asset.name), siteUrl).href,
      });
    }
    cursor = page.nextCursor;
  } while (cursor);

  return entries;
}

export default function sitemap(): Promise<MetadataRoute.Sitemap> {
  return buildSitemap(fetchAssets);
}
