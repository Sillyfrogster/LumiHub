import { notFound, permanentRedirect } from "next/navigation";
import { fetchLegacyAsset } from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";

export default async function LegacyAssetAddress({
  params,
}: {
  params: Promise<{ profile: string; name: string }>;
}) {
  const { profile, name } = await params;
  const asset = await fetchLegacyAsset(
    decodeURIComponent(profile),
    decodeURIComponent(name),
  );
  if (!asset) notFound();
  permanentRedirect(assetHref(asset.id, asset.name));
}
