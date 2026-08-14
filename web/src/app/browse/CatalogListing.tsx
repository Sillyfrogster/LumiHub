import Image from "next/image";
import browseCatalogLumi from "@/assets/art/full/browse-catalog-lumi.webp";
import { Shell } from "@/components/layout/Shell";
import type { BrowseFilters, BrowsePage } from "@/lib/api/query";
import { BrowseResults } from "./BrowseResults";
import styles from "./page.module.css";

export function CatalogListing({
  title,
  introduction,
  filters,
  initialPage,
}: {
  title: string;
  introduction: string;
  filters: BrowseFilters;
  initialPage: BrowsePage | null;
}) {
  return (
    <div className={styles.page}>
      <section className={styles.masthead}>
        <div className={styles.lumiArt}>
          <Image
            src={browseCatalogLumi}
            alt=""
            fill
            priority
            sizes="(max-width: 760px) 100vw, 66vw"
          />
        </div>
        <Shell className={styles.mastheadInner}>
          <div className={styles.intro}>
            <h1>{title}</h1>
            <p>{introduction}</p>
          </div>
        </Shell>
      </section>
      <BrowseResults filters={filters} initialPage={initialPage} />
    </div>
  );
}
