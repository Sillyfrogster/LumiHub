import { Art } from "@/components/art/Art";
import { CreationCard } from "@/components/creations/CreationCard";
import { Shell } from "@/components/layout/Shell";
import { SectionHead } from "@/components/ui/SectionHead";
import { FEATURED } from "@/data/creations";
import styles from "./FeaturedShelf.module.css";

export function FeaturedShelf() {
  return (
    <Shell as="section" className={styles.section}>
      <SectionHead
        eyebrow="Featured this week"
        title="Freshly bound, freshly loved"
        action={{ label: "View all", href: "/browse" }}
      />

      <Art name="rule-flower" width={420} className={styles.rule} />

      <ul className={styles.grid}>
        {FEATURED.map((creation) => (
          <li key={creation.id}>
            <CreationCard
              creation={creation}
              size="large"
              metrics={["downloads", "favorites", "comments"]}
            />
          </li>
        ))}
      </ul>
    </Shell>
  );
}
