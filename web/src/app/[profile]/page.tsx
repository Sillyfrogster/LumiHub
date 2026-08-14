import { cookies } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { fetchAssets, fetchDeletedAssets, fetchProfile } from "@/lib/api/query";
import { buildBrowseHref, readBrowseFilters } from "@/lib/browse-url";
import { ProfileListing } from "./ProfileListing";

export default async function CreatorProfileListing({
  params,
  searchParams,
}: {
  params: Promise<{ profile: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { profile: encodedProfile } = await params;
  const requestedProfile = decodeURIComponent(encodedProfile);
  if (!requestedProfile.startsWith("@")) notFound();

  const requestedHandle = requestedProfile.slice(1);
  const handle = requestedHandle.toLowerCase();
  const [filters, profile, cookie] = await Promise.all([
    searchParams.then(readBrowseFilters),
    fetchProfile(handle),
    cookies().then((value) => value.toString()),
  ]);
  if (!profile) notFound();

  const basePath = `/@${profile.handle}`;
  if (requestedHandle !== profile.handle) {
    redirect(buildBrowseHref(filters, basePath));
  }

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
