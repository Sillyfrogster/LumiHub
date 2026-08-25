import Link from "next/link";
import { Shell } from "@/components/layout/Shell";
import { CreatorMark } from "@/components/media/CreatorMark";
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
  const isOwner = deletedAssets !== null;

  return (
    <div className={styles.page}>
      <header className={styles.profileHeader}>
        <div className={styles.profileArt} aria-hidden="true" />
        <Shell className={styles.profileHeaderInner}>
          <CreatorMark handle={profile.handle} />
          <div className={styles.identity}>
            <h1 data-long={profile.handle.length > 20 || undefined}>
              @{profile.handle}
            </h1>
            <div className={styles.scope}>
              <strong>{isOwner ? "Owner view" : "Public profile"}</strong>
              <span>
                {isOwner
                  ? "All active work and recoverable deletions"
                  : "Published work"}
              </span>
            </div>
          </div>
        </Shell>
      </header>

      {deletedAssets !== null ? (
        <div className={styles.sectionBar}>
          <Shell>
            <nav className={styles.sections} aria-label="Profile sections">
              <Link href={basePath} aria-current="page">
                Creations
              </Link>
              <Link href="#deleted">
                Deleted
                <span>{deletedAssets.length}</span>
              </Link>
            </nav>
          </Shell>
        </div>
      ) : null}

      <div className={styles.work}>
        <BrowseResults
          filters={filters}
          initialPage={initialPage}
          creator={profile.handle}
          basePath={basePath}
          heading={isOwner ? "Your creations" : "Published work"}
        />
      </div>
      {deletedAssets !== null ? (
        <div className={styles.deletedRegion}>
          <DeletedAssets initialItems={deletedAssets} />
        </div>
      ) : null}
    </div>
  );
}
