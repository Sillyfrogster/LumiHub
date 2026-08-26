import type { Metadata } from "next";
import { cookies } from "next/headers";
import { fetchAssets } from "@/lib/api/query";
import { readBrowseFilters } from "@/lib/browse-url";
import { KIND_LABELS } from "@/lib/kinds";
import { pageMetadata } from "@/lib/site-metadata";
import { CatalogListing } from "./CatalogListing";

export async function generateMetadata({
  searchParams,
}: PageProps<"/browse">): Promise<Metadata> {
  const filters = readBrowseFilters(await searchParams);
  const subject = filters.kind
    ? `${KIND_LABELS[filters.kind].toLowerCase()}s`
    : "characters, lorebooks, presets, themes and packs";

  if (filters.q) {
    return pageMetadata(
      `${filters.q} \u00b7 Browse`,
      `Illarin ${subject} matching ${filters.q}.`,
    );
  }
  return pageMetadata("Browse", `Every one of Illarin's ${subject}.`);
}

export default async function BrowsePage({
  searchParams,
}: PageProps<"/browse">) {
  const filters = readBrowseFilters(await searchParams);
  const cookie = (await cookies()).toString();
  const initialPage = await fetchAssets(
    { ...filters, limit: 24 },
    cookie,
  ).catch(() => null);

  return (
    <CatalogListing
      title="Browse"
      introduction="Find characters, lorebooks, presets, themes, and packs. Narrow by kind, the app you use, or what the asset includes."
      filters={filters}
      initialPage={initialPage}
    />
  );
}
