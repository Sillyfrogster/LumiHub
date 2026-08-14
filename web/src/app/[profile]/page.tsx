import { cookies } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { fetchAssets, fetchProfile } from "@/lib/api/query";
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
  const filters = readBrowseFilters(await searchParams);
  const handle = requestedHandle.toLowerCase();
  const profile = await fetchProfile(handle);
  if (!profile) notFound();

  const basePath = `/@${profile.handle}`;
  if (requestedHandle !== profile.handle) {
    redirect(buildBrowseHref(filters, basePath));
  }

  const cookie = (await cookies()).toString();
  const initialPage = await fetchAssets(
    { ...filters, creator: profile.handle, limit: 24 },
    cookie,
  ).catch(() => null);

  return (
    <ProfileListing
      profile={profile}
      filters={filters}
      initialPage={initialPage}
    />
  );
}
