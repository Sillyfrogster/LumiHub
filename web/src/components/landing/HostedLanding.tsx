import type { BrowseAsset, BrowsePage, NsfwVisibility } from "@/lib/api/query";
import { AssetPageChapter } from "./AssetPageChapter";
import { CatalogChapter } from "./CatalogChapter";
import { CloseChapter } from "./CloseChapter";
import { ConvergenceHero } from "./ConvergenceHero";
import { DeliveryChapter } from "./DeliveryChapter";
import styles from "./HostedLanding.module.css";

type HostedLandingProps = {
  assets: BrowseAsset[];
  platforms: BrowsePage["platforms"];
  visibility: NsfwVisibility;
  suppressed: number;
  emptyState: BrowsePage["emptyState"];
  unavailable: boolean;
};

export function HostedLanding({
  assets,
  platforms,
  visibility,
  suppressed,
  emptyState,
  unavailable,
}: HostedLandingProps) {
  return (
    <div className={styles.page}>
      <ConvergenceHero assets={assets} platforms={platforms} />
      <CatalogChapter
        assets={assets}
        visibility={visibility}
        suppressed={suppressed}
        emptyState={emptyState}
        unavailable={unavailable}
      />
      <AssetPageChapter />
      <DeliveryChapter platforms={platforms} />
      <CloseChapter />
    </div>
  );
}
