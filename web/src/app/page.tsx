import { CreatorSection } from "@/components/landing/CreatorSection";
import { FeaturedShelf } from "@/components/landing/FeaturedShelf";
import { Hero } from "@/components/landing/Hero";
import { Newsletter } from "@/components/landing/Newsletter";
import { StatsBand } from "@/components/landing/StatsBand";
import { TypeShowcase } from "@/components/landing/TypeShowcase";

export default function LandingPage() {
  return (
    <>
      <Hero />
      <FeaturedShelf />
      <TypeShowcase />
      <StatsBand />
      <CreatorSection />
      <Newsletter />
    </>
  );
}
