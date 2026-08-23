import { cookies } from "next/headers";
import { notFound, permanentRedirect, redirect } from "next/navigation";
import { fetchAssets, fetchDeletedAssets, fetchProfile } from "@/lib/api/query";
import { buildBrowseHref, readBrowseFilters } from "@/lib/browse-url";
import { readProfileAddress } from "@/lib/profile-address";
import { ProfileListing } from "./ProfileListing";

export default async function CreatorProfileListing({
  params,
  searchParams,
}: {
  params: Promise<{ profile: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { profile: encodedProfile } = await params;
  const requested = decodeURIComponent(encodedProfile);
  const address = readProfileAddress(requested);
  if (!address) notFound();

  const [filters, profile, cookie] = await Promise.all([
    searchParams.then(readBrowseFilters),
    fetchProfile(address.handle),
    cookies().then((value) => value.toString()),
  ]);
  if (!profile) notFound();

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
