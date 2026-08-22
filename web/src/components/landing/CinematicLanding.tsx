"use client";

import {
  ArrowDownToLine,
  ArrowRight,
  BookOpenText,
  Boxes,
  Cable,
  Layers,
  PackageOpen,
  Palette,
  SlidersHorizontal,
  Upload,
  UserRound,
} from "lucide-react";
import dynamic from "next/dynamic";
import Image from "next/image";
import Link from "next/link";
import {
  type CSSProperties,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import mascot from "@/assets/art/full/illarin-mascot-refractive-v1.webp";
import type { BrowseAsset, BrowsePage, NsfwVisibility } from "@/lib/api/query";
import { assetHref } from "@/lib/asset-url";
import { KIND_LABELS } from "@/lib/kinds";
import styles from "./CinematicLanding.module.css";

const LandingCanvas = dynamic(
  () => import("./LandingCanvas").then((module) => module.LandingCanvas),
  { ssr: false },
);

const CINEMATIC_PREFERENCE = "illarin-cinematic:v1";

const KIND_ICON = {
  character: UserRound,
  lorebook: BookOpenText,
  preset: SlidersHorizontal,
  theme: Palette,
  pack: PackageOpen,
} as const;

/** Each chapter sits on the scroll position where its movement resolves. */
const CHAPTERS = [
  { label: "Catalog", at: 0 },
  { label: "Kinds", at: 0.334 },
  { label: "Build", at: 0.667 },
  { label: "Take", at: 0.97 },
] as const;

type EmptyState = BrowsePage["emptyState"];
type LandingStyle = CSSProperties & Record<`--${string}`, string | number>;

type CinematicLandingProps = {
  assets: BrowseAsset[];
  visibility: NsfwVisibility;
  suppressed: number;
  emptyState: EmptyState;
  unavailable: boolean;
};

function clamp(value: number, minimum = 0, maximum = 1) {
  return Math.min(maximum, Math.max(minimum, value));
}

function range(progress: number, start: number, peak: number, end: number) {
  if (progress <= start || progress >= end) return 0;
  if (progress === peak) return 1;
  return progress < peak
    ? clamp((progress - start) / (peak - start))
    : clamp((end - progress) / (end - peak));
}

function panelStyle(opacity: number, staticMode: boolean): LandingStyle {
  const visible = staticMode ? 1 : opacity;
  return {
    opacity: visible,
    pointerEvents: visible > 0.68 ? "auto" : "none",
    transform: staticMode
      ? "none"
      : `translate3d(0, ${(1 - visible) * 22}px, 0)`,
  };
}

function CatalogThumb({ asset }: { asset: BrowseAsset }) {
  const [failed, setFailed] = useState(false);
  const Icon = KIND_ICON[asset.kind];

  return (
    <span className={styles.thumb} data-kind={asset.kind} aria-hidden="true">
      {asset.cover && !failed ? (
        <Image
          src={asset.cover.url}
          alt=""
          fill
          sizes="72px"
          className={styles.thumbImage}
          onError={() => setFailed(true)}
          unoptimized
        />
      ) : (
        <span className={styles.thumbFallback}>
          <Icon size={19} strokeWidth={1.2} />
          <span>{asset.kind.slice(0, 3)}</span>
        </span>
      )}
    </span>
  );
}

function CatalogMessage({
  unavailable,
  emptyState,
  suppressed,
}: Pick<CinematicLandingProps, "unavailable" | "emptyState" | "suppressed">) {
  let message = "The catalog is waiting for its first published work.";

  if (unavailable) {
    message = "The catalog could not be reached just now.";
  } else if (emptyState === "suppressed" || suppressed > 0) {
    message = "Published work is outside your current content preference.";
  }

  return (
    <div className={styles.catalogMessage}>
      <p>{message}</p>
      <Link href="/browse">Open the catalog</Link>
    </div>
  );
}

function ChapterPanel({
  className,
  style,
  children,
}: {
  className: string;
  style: LandingStyle;
  children: ReactNode;
}) {
  return (
    <div className={`${styles.panel} ${className}`} style={style}>
      {children}
    </div>
  );
}

export function CinematicLanding({
  assets,
  visibility,
  suppressed,
  emptyState,
  unavailable,
}: CinematicLandingProps) {
  const journeyRef = useRef<HTMLElement>(null);
  const [progress, setProgress] = useState(0);
  const [preference, setPreference] = useState<boolean | null>(null);
  const [sceneFailed, setSceneFailed] = useState(false);

  useEffect(() => {
    setPreference(localStorage.getItem(CINEMATIC_PREFERENCE) !== "disabled");
  }, []);

  const cinematic = preference !== false && !sceneFailed;
  const staticMode = !cinematic;

  useEffect(() => {
    if (!cinematic) {
      setProgress(0);
      return;
    }

    let frame = 0;
    const measure = () => {
      frame = 0;
      const journey = journeyRef.current;
      if (!journey) return;
      const bounds = journey.getBoundingClientRect();
      const travel = Math.max(1, journey.offsetHeight - window.innerHeight);
      setProgress(clamp(-bounds.top / travel));
    };
    const schedule = () => {
      if (frame) return;
      frame = requestAnimationFrame(measure);
    };

    measure();
    window.addEventListener("scroll", schedule, { passive: true });
    window.addEventListener("resize", schedule);
    return () => {
      window.removeEventListener("scroll", schedule);
      window.removeEventListener("resize", schedule);
      if (frame) cancelAnimationFrame(frame);
    };
  }, [cinematic]);

  const activeChapter =
    progress < 0.17 ? 0 : progress < 0.5 ? 1 : progress < 0.8 ? 2 : 3;

  // Copy peaks where its movement resolves: 0, 1/3, 2/3 and 1.
  const opacity = useMemo(
    () => [
      clamp(1 - progress / 0.16),
      range(progress, 0.18, 0.334, 0.48),
      range(progress, 0.52, 0.667, 0.81),
      clamp((progress - 0.85) / 0.12),
    ],
    [progress],
  );

  // Build is the one movement whose copy sits right, so the field opens there.
  const rightField = opacity[2];
  const leftField = Math.max(opacity[0], opacity[1], opacity[3]);

  const changeMode = useCallback(() => {
    const enable = !staticMode;
    if (enable) {
      localStorage.setItem(CINEMATIC_PREFERENCE, "disabled");
      setPreference(false);
      requestAnimationFrame(() => journeyRef.current?.scrollIntoView());
      return;
    }

    localStorage.setItem(CINEMATIC_PREFERENCE, "enabled");
    setSceneFailed(false);
    setPreference(true);
  }, [staticMode]);

  const seek = useCallback((target: number) => {
    const journey = journeyRef.current;
    if (!journey) return;
    const rootTop = window.scrollY + journey.getBoundingClientRect().top;
    const travel = Math.max(1, journey.offsetHeight - window.innerHeight);
    const reduceMotion = window.matchMedia(
      "(prefers-reduced-motion: reduce)",
    ).matches;
    window.scrollTo({
      top: rootTop + travel * target,
      behavior: reduceMotion ? "auto" : "smooth",
    });
  }, []);

  return (
    <section
      ref={journeyRef}
      className={styles.journey}
      data-cinematic={cinematic ? "on" : "off"}
    >
      <div className={styles.stickyStage}>
        {preference !== null && cinematic ? (
          <LandingCanvas
            progress={progress}
            enabled
            onFailure={() => setSceneFailed(true)}
          />
        ) : null}

        {staticMode ? (
          <div className={styles.host} aria-hidden="true">
            <Image
              src={mascot}
              alt=""
              fill
              priority
              sizes="(max-width: 760px) 112vw, 52vw"
              className={styles.hostImage}
            />
          </div>
        ) : null}

        <div
          className={`${styles.scrim} ${styles.scrimLeft}`}
          style={{ opacity: staticMode ? 1 : leftField }}
          aria-hidden="true"
        />
        <div
          className={`${styles.scrim} ${styles.scrimRight}`}
          style={{ opacity: staticMode ? 0 : rightField }}
          aria-hidden="true"
        />
        <div className={styles.floorScrim} aria-hidden="true" />

        <ChapterPanel
          className={styles.heroPanel}
          style={panelStyle(opacity[0], staticMode)}
        >
          <h1>
            AI roleplay,
            <br />
            <em>in one catalog.</em>
          </h1>
          <p>
            Characters, lorebooks, presets, themes and packs, kept together in
            one place instead of scattered across a dozen communities and apps.
          </p>
          <div className={styles.heroActions}>
            <Link href="/browse" className={styles.primaryAction}>
              Browse the catalog
              <ArrowRight size={16} strokeWidth={1.5} />
            </Link>
            <Link href="/upload" className={styles.secondaryAction}>
              <Upload size={15} strokeWidth={1.5} />
              Publish
            </Link>
          </div>
          <p className={styles.truthLine}>
            Five kinds of file. One catalog. Each kept as itself.
          </p>
        </ChapterPanel>

        <ChapterPanel
          className={styles.togetherPanel}
          style={panelStyle(opacity[1], staticMode)}
        >
          <h2>
            Five kinds.
            <br />
            <em>One place to look.</em>
          </h2>
          <p>
            A character is not a preset, and a lorebook is not a theme. They
            share one catalog without being forced into the same shape.
          </p>
          <ul className={styles.kindRun} aria-label="Catalog asset kinds">
            <li>Character</li>
            <li>Lorebook</li>
            <li>Preset</li>
            <li>Theme</li>
            <li>Pack</li>
          </ul>
        </ChapterPanel>

        <ChapterPanel
          className={styles.builderPanel}
          style={panelStyle(opacity[2], staticMode)}
        >
          <h2>
            The page you build
            <br />
            <em>is the page they read.</em>
          </h2>
          <p>
            Import a file you already have, or start from nothing. Arrange it in
            blocks — what you compose is what readers get.
          </p>
          <div className={styles.builderProof}>
            <span>
              <Boxes size={18} strokeWidth={1.3} />
              Blocks hold the sections
            </span>
            <span>
              <Layers size={18} strokeWidth={1.3} />
              Elements hold the work
            </span>
          </div>
        </ChapterPanel>

        <ChapterPanel
          className={styles.carryPanel}
          style={panelStyle(opacity[3], staticMode)}
        >
          <h2>
            Take it
            <br />
            <em>with you.</em>
          </h2>
          <p>
            Download an asset in the formats it can actually hold. Or link an
            app once, and send work straight there when you want it.
          </p>
          <div className={styles.outcomes}>
            <span>
              <ArrowDownToLine size={18} strokeWidth={1.3} />
              <strong>Download</strong>
              <small>Only the formats that fit</small>
            </span>
            <span>
              <Cable size={18} strokeWidth={1.3} />
              <strong>Linked app</strong>
              <small>Optional, never required</small>
            </span>
          </div>
          <Link href="/browse" className={styles.finalAction}>
            Enter the catalog
            <ArrowRight size={16} strokeWidth={1.5} />
          </Link>
        </ChapterPanel>

        <div
          className={styles.catalogShelf}
          style={panelStyle(clamp(1 - progress / 0.16), staticMode)}
        >
          <header className={styles.catalogHeader}>
            <div>
              <h2>Latest in the catalog</h2>
              <p>Creator work, presented on its own terms.</p>
            </div>
            <Link href="/browse">See everything</Link>
          </header>

          {assets.length > 0 ? (
            <ul className={styles.catalogItems}>
              {assets.map((asset) => (
                <li key={asset.id}>
                  <Link href={assetHref(asset.id, asset.name)}>
                    <CatalogThumb asset={asset} />
                    <span className={styles.assetIdentity}>
                      <small>
                        {KIND_LABELS[asset.kind]}
                        {asset.isNsfw
                          ? visibility === "shown"
                            ? " · Adult"
                            : " · Adult, blurred"
                          : ""}
                      </small>
                      <strong>{asset.name}</strong>
                      <span>@{asset.creator}</span>
                    </span>
                    <ArrowRight
                      className={styles.assetArrow}
                      size={15}
                      strokeWidth={1.3}
                      aria-hidden="true"
                    />
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

          {assets.length > 0 && suppressed > 0 ? (
            <p className={styles.suppressedNote}>
              {suppressed} more {suppressed === 1 ? "item is" : "items are"}{" "}
              outside your content preference.
            </p>
          ) : null}
        </div>

        <button
          type="button"
          className={styles.cinematicToggle}
          onClick={changeMode}
          aria-pressed={staticMode}
        >
          <span className={styles.toggleGlyph} aria-hidden="true" />
          {sceneFailed
            ? "Retry cinematic"
            : cinematic
              ? "Disable cinematic"
              : "Enable cinematic"}
        </button>

        {cinematic ? (
          <nav className={styles.chapterNav} aria-label="Landing chapters">
            {CHAPTERS.map((chapter, index) => (
              <button
                key={chapter.label}
                type="button"
                data-active={activeChapter === index || undefined}
                onClick={() => seek(chapter.at)}
              >
                <span>{chapter.label}</span>
                <i aria-hidden="true" />
              </button>
            ))}
          </nav>
        ) : null}

        {sceneFailed ? (
          <output className={styles.sceneStatus}>
            The still composition is active because the cinematic could not be
            loaded.
          </output>
        ) : null}
      </div>
    </section>
  );
}
