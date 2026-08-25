import { cookies } from "next/headers";
import { HostedLanding } from "@/components/landing/HostedLanding";
import { fetchAssets } from "@/lib/api/query";

export default async function LandingPage() {
  const cookie = (await cookies()).toString();
  const latest = await fetchAssets({ limit: 9 }, cookie).catch(() => null);

  return (
    <HostedLanding
      assets={latest?.items ?? []}
      visibility={latest?.visibility ?? "blurred"}
      suppressed={latest?.suppressed ?? 0}
      emptyState={latest?.emptyState ?? null}
      unavailable={!latest}
    />
  );
}
