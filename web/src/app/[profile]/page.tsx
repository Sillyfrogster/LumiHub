import type { Metadata } from "next";
import { cookies } from "next/headers";
import { notFound, permanentRedirect, redirect } from "next/navigation";
import { cache } from "react";
import { fetchAssets, fetchDeletedAssets, fetchProfile } from "@/lib/api/query";
import { buildBrowseHref, readBrowseFilters } from "@/lib/browse-url";
import { readProfileAddress } from "@/lib/profile-address";
import { pageMetadata } from "@/lib/site-metadata";
import { ProfileListing } from "./ProfileListing";

const loadProfile = cache(async (segment: string) => {
  const address = readProfileAddress(decodeURIComponent(segment));
  return address
    ? { address, profile: await fetchProfile(address.handle) }
    : null;
});

export async function generateMetadata({
  params,
}: {
  params: Promise<{ profile: string }>;
}): Promise<Metadata> {
  const found = await loadProfile((await params).profile);
  if (!found?.profile) return { title: "Not found" };

  const { profile } = found;
  const metadata = pageMetadata(
    `@${profile.handle}`,
    `Characters, lorebooks, presets, themes and packs published by ${profile.handle} on Illarin.`,
  );
  return { ...metadata, alternates: { canonical: `/@${profile.handle}` } };
}

export default async function CreatorProfileListing({
  params,
  searchParams,
}: {
  params: Promise<{ profile: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { profile: encodedProfile } = await params;
  const requested = decodeURIComponent(encodedProfile);
  const [filters, found, cookie] = await Promise.all([
    searchParams.then(readBrowseFilters),
    loadProfile(encodedProfile),
    cookies().then((value) => value.toString()),
  ]);
  if (!found?.profile) notFound();

  const { address, profile } = found;

  const canonical = `@${profile.handle}`;
  const here = buildBrowseHref(filters, `/${canonical}`);
  if (address.form === "legacy") permanentRedirect(here);
  if (requested !== canonical) redirect(here);

  const [initialPage, deletedAssets] = await Promise.all([
    fetchAssets(
      { ...filters, creator: profile.handle, limit: 24 },
      cookie,
    ).catch(() => null),
    fetchDeletedAssets(profile.handle, cookie),
  ]);

  return (
    <ProfileListing
      profile={profile}
      filters={filters}
      initialPage={initialPage}
      deletedAssets={deletedAssets}
    />
  );
}
