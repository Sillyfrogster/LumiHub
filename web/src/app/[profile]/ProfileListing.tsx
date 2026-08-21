import Link from "next/link";
import { Shell } from "@/components/layout/Shell";
import type {
  BrowseFilters,
  BrowsePage,
  DeletedAsset,
  Profile,
} from "@/lib/api/query";
import { BrowseResults } from "../browse/BrowseResults";
import { DeletedAssets } from "./DeletedAssets";
import styles from "./ProfileListing.module.css";

export function ProfileListing({
  profile,
  filters,
  initialPage,
  deletedAssets,
}: {
  profile: Profile;
  filters: BrowseFilters;
  initialPage: BrowsePage | null;
  deletedAssets: DeletedAsset[] | null;
}) {
  const basePath = `/@${profile.handle}`;
  const initial = profile.handle.slice(0, 1).toUpperCase();

  return (
    <div className={styles.page}>
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
              Assets published by @{profile.handle}.
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
            {deletedAssets ? <Link href="#deleted">Deleted</Link> : null}
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
      {deletedAssets ? <DeletedAssets initialItems={deletedAssets} /> : null}
    </div>
  );
}
