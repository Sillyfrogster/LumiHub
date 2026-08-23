import { redirectFromLegacyAssetAddress } from "@/lib/legacy-address";

export default async function LegacyThemeAddress({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  await redirectFromLegacyAssetAddress((await params).id);
}
