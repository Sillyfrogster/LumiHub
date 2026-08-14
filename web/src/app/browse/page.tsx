import { cookies } from "next/headers";
import Image from "next/image";
import city from "@/assets/art/city.webp";
import sprig from "@/assets/art/sprig.webp";
import { Shell } from "@/components/layout/Shell";
import {
  type BrowseFilters,
  type BrowseKind,
  fetchAssets,
} from "@/lib/api/query";
import { BrowseResults } from "./BrowseResults";
import styles from "./page.module.css";

const KINDS = new Set<BrowseKind>(["character", "lorebook", "preset", "theme"]);

function first(value: string | string[] | undefined) {
  return Array.isArray(value) ? value[0] : value;
}

function readFilters(
  values: Record<string, string | string[] | undefined>,
): BrowseFilters {
  const requestedKind = first(values.kind);
  const kind = KINDS.has(requestedKind as BrowseKind)
    ? (requestedKind as BrowseKind)
    : undefined;
  const q = first(values.q) || undefined;
  const platform = first(values.platform)?.trim() || undefined;
  const facets = Array.isArray(values.facet)
    ? values.facet
    : values.facet
      ? [values.facet]
      : undefined;

  return { kind, q, platform, facet: facets };
}

export default async function BrowsePage({
  searchParams,
}: PageProps<"/browse">) {
  const filters = readFilters(await searchParams);
  const cookie = (await cookies()).toString();
  const initialPage = await fetchAssets(
    { ...filters, limit: 24 },
    cookie,
  ).catch(() => null);

  return (
    <div className={styles.page}>
      <section className={styles.masthead}>
        <Shell className={styles.mastheadInner}>
          <div className={styles.intro}>
            <h1>Browse the collection</h1>
            <p>
              Find characters, lorebooks, presets, and themes made for stories
              still unfolding.
            </p>
          </div>
          <Image
            src={city}
            alt=""
            className={styles.city}
            priority
            sizes="(max-width: 720px) 72vw, 520px"
          />
          <Image src={sprig} alt="" className={styles.sprig} sizes="220px" />
        </Shell>
      </section>
      <BrowseResults filters={filters} initialPage={initialPage} />
    </div>
  );
}
