import { cookies } from "next/headers";
import { fetchAssets } from "@/lib/api/query";
import { readBrowseFilters } from "@/lib/browse-url";
import { CatalogListing } from "./CatalogListing";

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
      title="Browse the collection"
      introduction="Find characters, lorebooks, presets, and themes made for stories still unfolding."
      filters={filters}
      initialPage={initialPage}
    />
  );
}
