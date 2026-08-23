import { cookies } from "next/headers";
import { notFound, permanentRedirect } from "next/navigation";
import { fetchAsset, fetchLegacyProfile } from "@/lib/api/query";
import { assetHref, isAssetId } from "@/lib/asset-url";

/** Sends a v1 asset address to the permalink, resolving first so a hidden asset is the plain 404. */
export async function redirectFromLegacyAssetAddress(
  id: string,
): Promise<void> {
  if (!isAssetId(id)) notFound();
  const cookie = (await cookies()).toString();
  const asset = await fetchAsset(id, cookie);
  if (!asset) notFound();
  permanentRedirect(assetHref(asset.id, asset.name));
}

/** Sends v1's /user/<discordId> address to the profile that Discord identity belongs to. */
export async function redirectFromLegacyUserAddress(
  discordId: string,
): Promise<void> {
  const profile = await fetchLegacyProfile(discordId);
  if (!profile) notFound();
  permanentRedirect(`/@${profile.handle}`);
}
