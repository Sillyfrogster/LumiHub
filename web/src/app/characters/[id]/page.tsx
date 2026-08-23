import { redirectFromLegacyAssetAddress } from "@/lib/legacy-address";

export default async function LegacyCharacterAddress({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  await redirectFromLegacyAssetAddress((await params).id);
}
