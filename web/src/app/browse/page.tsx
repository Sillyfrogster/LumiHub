import { cookies } from "next/headers";
import Image from "next/image";
import heroLumi from "@/assets/art/full/hero-lumi.webp";
import { Shell } from "@/components/layout/Shell";
import { fetchAssets } from "@/lib/api/query";
import { readBrowseFilters } from "@/lib/browse-url";
import { BrowseResults } from "./BrowseResults";
import styles from "./page.module.css";

export default async function BrowsePage({
  searchParams,
}: PageProps<"/browse">) {
  const filters = readBrowseFilters(await searchParams);
  const cookie = (await cookies()).toString();
  const initialPage = await fetchAssets(
    { ...filters, limit: 24 },
    cookie,
  ).catch(() => null);

  return (
    <div className={styles.page}>
      <section className={styles.masthead}>
        <div className={styles.lumiArt}>
          <Image
            src={heroLumi}
            alt=""
            fill
            priority
            sizes="(max-width: 760px) 100vw, 58vw"
          />
        </div>
        <Shell className={styles.mastheadInner}>
          <div className={styles.intro}>
            <h1>Browse the collection</h1>
            <p>
              Find characters, lorebooks, presets, and themes made for stories
              still unfolding.
            </p>
          </div>
        </Shell>
      </section>
      <BrowseResults filters={filters} initialPage={initialPage} />
    </div>
  );
}
