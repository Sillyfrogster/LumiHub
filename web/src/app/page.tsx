import { cookies } from "next/headers";
import { CreatorSection } from "@/components/landing/CreatorSection";
import { FeaturedShelf } from "@/components/landing/FeaturedShelf";
import { Hero } from "@/components/landing/Hero";
import { fetchAssets } from "@/lib/api/query";

export default async function LandingPage() {
  const cookie = (await cookies()).toString();
  const latest = await fetchAssets({ limit: 4 }, cookie).catch(() => null);

  return (
    <>
      <Hero />
      <FeaturedShelf
        assets={latest?.items ?? []}
        visibility={latest?.visibility ?? "blurred"}
        unavailable={!latest}
      />
      <CreatorSection />
    </>
  );
}
