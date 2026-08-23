import { redirectFromLegacyUserAddress } from "@/lib/legacy-address";

export default async function LegacyUserAddress({
  params,
}: {
  params: Promise<{ discordId: string }>;
}) {
  await redirectFromLegacyUserAddress((await params).discordId);
}
