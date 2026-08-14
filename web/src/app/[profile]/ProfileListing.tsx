import Link from "next/link";
import { Shell } from "@/components/layout/Shell";
import type { BrowseFilters, BrowsePage, Profile } from "@/lib/api/query";
import { BrowseResults } from "../browse/BrowseResults";
import styles from "./ProfileListing.module.css";

export function ProfileListing({
  profile,
  filters,
  initialPage,
}: {
  profile: Profile;
  filters: BrowseFilters;
  initialPage: BrowsePage | null;
}) {
  const basePath = `/@${profile.handle}`;
  const initial = profile.handle.slice(0, 1).toUpperCase();

  return (
    <main className={styles.page}>
      <header className={styles.profileHeader}>
        <Shell className={styles.profileHeaderInner}>
          <div className={styles.portrait} aria-hidden="true">
            <span>{initial}</span>
          </div>
          <div className={styles.identity}>
            <h1 data-long={profile.handle.length > 20 || undefined}>
              @{profile.handle}
            </h1>
            <p className={styles.introduction}>
              A personal shelf of characters, lorebooks, presets, and themes.
            </p>
          </div>
        </Shell>
      </header>

      <div className={styles.sectionBar}>
        <Shell>
          <nav className={styles.sections} aria-label="Profile sections">
            <Link href={basePath} aria-current="page">
              Creations
            </Link>
          </nav>
        </Shell>
      </div>

      <BrowseResults
        filters={filters}
        initialPage={initialPage}
        creator={profile.handle}
        basePath={basePath}
        heading="Latest creations"
      />
    </main>
  );
}
