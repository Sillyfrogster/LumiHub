"use client";

import gsap from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";
import { ArrowRight } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useEffect, useRef } from "react";
import { Shell } from "@/components/layout/Shell";
import { DefaultCover } from "@/components/media/DefaultCover";
import type { BrowseAsset, BrowsePage, NsfwVisibility } from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";
import { KIND_LABELS } from "@/lib/kinds";
import { withCoversFirst } from "@/lib/landing-work";
import styles from "./CatalogChapter.module.css";

type CatalogChapterProps = {
  assets: BrowseAsset[];
  visibility: NsfwVisibility;
  suppressed: number;
  emptyState: BrowsePage["emptyState"];
  unavailable: boolean;
};

export function CatalogChapter({
  assets,
  visibility,
  suppressed,
  emptyState,
  unavailable,
}: CatalogChapterProps) {
  const root = useRef<HTMLElement>(null);
  const shown = withCoversFirst(assets).slice(0, 5);

  useCatalogReveal(root, shown.length);

  return (
    <section
      className={styles.chapter}
      ref={root}
      aria-labelledby="landing-catalog-title"
    >
      <Shell className={styles.stage}>
        <header className={styles.head}>
          <h2 id="landing-catalog-title" data-lift>
            Recently published
          </h2>
          <Link href="/browse" className={styles.seeAll} data-lift>
            See the full catalog
            <ArrowRight size={15} strokeWidth={1.5} aria-hidden="true" />
          </Link>
        </header>

        {shown.length > 0 ? (
          <ul className={styles.rank}>
            {shown.map((asset, index) => (
              <li
                key={asset.id}
                data-piece
                data-lead={index === 0 || undefined}
              >
                <Link
                  href={assetHref(asset.id, asset.name)}
                  className={styles.piece}
                >
                  <span className={styles.well}>
                    {asset.cover ? (
                      <Image
                        src={asset.cover.url}
                        alt=""
                        fill
                        sizes="(max-width: 900px) 50vw, 30vw"
                        className={styles.image}
                        unoptimized
                      />
                    ) : (
                      <DefaultCover kind={asset.kind} compact={index !== 0} />
                    )}
                  </span>
                  <span className={styles.identity}>
                    <strong>{asset.name || "Untitled"}</strong>
                    <span>
                      @{asset.creator} · {KIND_LABELS[asset.kind]}
                      {asset.isNsfw
                        ? visibility === "shown"
                          ? " · Adult"
                          : " · Adult, blurred"
                        : ""}
                    </span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        ) : (
          <CatalogMessage
            unavailable={unavailable}
            emptyState={emptyState}
            suppressed={suppressed}
          />
        )}

        {shown.length > 0 && suppressed > 0 ? (
          <p className={styles.suppressed} data-lift>
            {suppressed} more {suppressed === 1 ? "item is" : "items are"}{" "}
            outside your content preference.
          </p>
        ) : null}
      </Shell>
    </section>
  );
}

function CatalogMessage({
  unavailable,
  emptyState,
  suppressed,
}: Pick<CatalogChapterProps, "unavailable" | "emptyState" | "suppressed">) {
  let message = "The catalog is waiting for its first published work.";

  if (unavailable) {
    message = "The catalog could not be reached just now.";
  } else if (emptyState === "suppressed" || suppressed > 0) {
    message = "Published work is outside your current content preference.";
  }

  return (
    <div className={styles.message} data-lift>
      <p>{message}</p>
      <Link href="/browse">
        Open the catalog
        <ArrowRight size={15} strokeWidth={1.5} aria-hidden="true" />
      </Link>
    </div>
  );
}

/** The lead piece lands first and the rank fills outward from it */
function useCatalogReveal(
  root: React.RefObject<HTMLElement | null>,
  count: number,
) {
  useEffect(() => {
    const node = root.current;
    if (!node || count === 0) return;

    gsap.registerPlugin(ScrollTrigger);
    const media = gsap.matchMedia();

    media.add("(prefers-reduced-motion: no-preference)", () => {
      const pieces = node.querySelectorAll("[data-piece]");
      const lifts = node.querySelectorAll("[data-lift]");

      gsap.from(lifts, {
        opacity: 0,
        y: 20,
        duration: 0.6,
        stagger: 0.09,
        ease: "power3.out",
        scrollTrigger: { trigger: node, start: "top 78%", once: true },
      });

      gsap.from(pieces, {
        opacity: 0,
        y: 46,
        scale: 0.94,
        duration: 0.78,
        ease: "power3.out",
        stagger: { each: 0.085, from: "start" },
        scrollTrigger: { trigger: node, start: "top 68%", once: true },
      });
    });

    return () => {
      media.revert();
    };
  }, [root, count]);
}
