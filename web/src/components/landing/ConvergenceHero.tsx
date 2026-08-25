"use client";

import gsap from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";
import { ArrowRight } from "lucide-react";
import Image from "next/image";
import { Fragment, useEffect, useRef } from "react";
import { Shell } from "@/components/layout/Shell";
import { DefaultCover } from "@/components/media/DefaultCover";
import { Button } from "@/components/ui/Button";
import type { BrowseAsset } from "@/lib/api/query";
import { KIND_LABELS } from "@/lib/kinds";
import { withCoversFirst } from "@/lib/landing-work";
import styles from "./ConvergenceHero.module.css";

const HEADING_LINES = [
  ["Gather", "it", "here."],
  ["Use", "it", "anywhere."],
] as const;

const HERO_NOTES = [
  { term: "Source file", detail: "Kept exactly as uploaded." },
  { term: "Exports", detail: "Only the formats the content can carry." },
  { term: "Delivery", detail: "A download, or a send to a linked app." },
] as const;

/** Resting depth of each shelf plane, furthest first, focal plane in the middle */
const PLANE_DEPTHS = [
  { x: -212, z: -400, rotate: 33, scale: 0.72 },
  { x: -118, z: -200, rotate: 20, scale: 0.86 },
  { x: 0, z: 0, rotate: 0, scale: 1 },
  { x: 118, z: -200, rotate: -20, scale: 0.86 },
  { x: 212, z: -400, rotate: -33, scale: 0.72 },
] as const;

/** Where each plane starts before it gathers, relative to its resting depth */
const PLANE_SCATTER = [
  { x: -300, y: -150, z: -680, rotate: 34 },
  { x: -190, y: 120, z: -520, rotate: 22 },
  { x: 0, y: 40, z: -820, rotate: 0 },
  { x: 190, y: 130, z: -520, rotate: -22 },
  { x: 300, y: -140, z: -680, rotate: -34 },
] as const;

type ConvergenceHeroProps = {
  assets: BrowseAsset[];
};

export function ConvergenceHero({ assets }: ConvergenceHeroProps) {
  const root = useRef<HTMLElement>(null);
  const planes = pickPlanes(assets);

  useHeroChoreography(root);

  return (
    <section className={styles.hero} ref={root}>
      <div className={styles.field} data-field aria-hidden="true" />

      <Shell className={styles.stage}>
        <div className={styles.copy}>
          <h1 className={styles.statement}>
            {HEADING_LINES.map((line) => (
              <span className={styles.line} key={line.join(" ")}>
                {line.map((word, index) => (
                  <Fragment key={word}>
                    {index > 0 ? " " : null}
                    <span className={styles.mask}>
                      <span className={styles.word} data-word>
                        {word}
                      </span>
                    </span>
                  </Fragment>
                ))}
              </span>
            ))}
          </h1>

          <p className={styles.lede} data-enter>
            Characters, lorebooks, presets and themes, whichever application
            they were built for.
          </p>

          <div className={styles.actions} data-enter>
            <Button href="/browse" size="large">
              Browse the catalog
              <ArrowRight size={16} strokeWidth={1.5} aria-hidden="true" />
            </Button>
            <Button href="/upload" variant="outline" size="large">
              Publish
            </Button>
          </div>

          <dl
            className={styles.notes}
            data-enter
            aria-label="What Illarin does with a published file"
          >
            {HERO_NOTES.map(({ term, detail }) => (
              <div key={term}>
                <dt>{term}</dt>
                <dd>{detail}</dd>
              </div>
            ))}
          </dl>
        </div>

        <div className={styles.shelf} data-shelf aria-hidden="true">
          <div className={styles.depth} data-depth>
            {planes.map((asset, index) => (
              <ShelfPlane
                key={asset.id}
                asset={asset}
                index={index}
                focal={index === 2}
              />
            ))}
          </div>
        </div>
      </Shell>
    </section>
  );
}

function ShelfPlane({
  asset,
  index,
  focal,
}: {
  asset: BrowseAsset;
  index: number;
  focal: boolean;
}) {
  const depth = PLANE_DEPTHS[index];

  return (
    <article
      className={styles.plane}
      data-plane
      data-focal={focal || undefined}
      style={
        {
          "--x": `${depth.x}px`,
          "--z": `${depth.z}px`,
          "--rotate": `${depth.rotate}deg`,
          "--scale": depth.scale,
        } as React.CSSProperties
      }
    >
      <div className={styles.planeCard}>
        <div className={styles.planeWell}>
          {asset.cover ? (
            <Image
              src={asset.cover.url}
              alt=""
              fill
              sizes="260px"
              className={styles.planeImage}
              unoptimized
            />
          ) : (
            <DefaultCover kind={asset.kind} compact />
          )}
        </div>
        {focal ? (
          <footer className={styles.planeIdentity}>
            <small>{KIND_LABELS[asset.kind]}</small>
            <strong>{asset.name || "Untitled"}</strong>
            <span>@{asset.creator}</span>
          </footer>
        ) : null}
      </div>
    </article>
  );
}

/** Puts a real cover on the focal plane, since it is the one the eye lands on */
function pickPlanes(assets: BrowseAsset[]): BrowseAsset[] {
  const shelf = withCoversFirst(assets).slice(0, 5);
  if (shelf.length < 5) return shelf;

  return [shelf[3], shelf[1], shelf[0], shelf[2], shelf[4]];
}

/** Gathers the shelf on entry, then drifts the field on scroll */
function useHeroChoreography(root: React.RefObject<HTMLElement | null>) {
  useEffect(() => {
    const node = root.current;
    if (!node) return;

    gsap.registerPlugin(ScrollTrigger);

    const media = gsap.matchMedia();

    media.add(
      {
        motion: "(prefers-reduced-motion: no-preference)",
        still: "(prefers-reduced-motion: reduce)",
        wide: "(min-width: 900px)",
      },
      (context) => {
        const { motion, wide } = context.conditions as {
          motion: boolean;
          wide: boolean;
        };

        const words = node.querySelectorAll("[data-word]");
        const entering = node.querySelectorAll("[data-enter]");
        const planes = node.querySelectorAll("[data-plane]");
        const field = node.querySelector("[data-field]");

        if (!motion) {
          gsap.set([words, entering, planes, field], {
            clearProps: "all",
            opacity: 1,
          });
          return;
        }

        const entry = gsap.timeline({ defaults: { ease: "power3.out" } });

        entry
          .from(field, { opacity: 0, scale: 1.04, duration: 0.9 }, 0)
          .from(words, { yPercent: 118, duration: 0.56, stagger: 0.042 }, 0.07)
          .from(
            entering,
            { opacity: 0, y: 18, duration: 0.5, stagger: 0.08 },
            0.32,
          );

        if (wide) {
          entry.from(
            planes,
            {
              opacity: 0,
              x: (index: number) => PLANE_SCATTER[index].x,
              y: (index: number) => PLANE_SCATTER[index].y,
              z: (index: number) => PLANE_SCATTER[index].z,
              rotationY: (index: number) => PLANE_SCATTER[index].rotate,
              duration: 0.82,
              ease: "back.out(1.15)",
              stagger: { each: 0.075, from: "edges" },
            },
            0.1,
          );

          gsap.to(field, {
            yPercent: -6,
            ease: "none",
            scrollTrigger: {
              trigger: node,
              start: "top top",
              end: "bottom top",
              scrub: 0.8,
            },
          });

          const depth = node.querySelector("[data-depth]");
          const shelf = node.querySelector("[data-shelf]");
          if (depth && shelf) {
            const tiltY = gsap.quickTo(depth, "rotationY", {
              duration: 0.6,
              ease: "power3.out",
            });
            const tiltX = gsap.quickTo(depth, "rotationX", {
              duration: 0.6,
              ease: "power3.out",
            });

            const track = (event: PointerEvent) => {
              const box = shelf.getBoundingClientRect();
              const px = (event.clientX - box.left) / box.width - 0.5;
              const py = (event.clientY - box.top) / box.height - 0.5;
              tiltY(px * 13);
              tiltX(py * -9);
            };
            const rest = () => {
              tiltY(0);
              tiltX(0);
            };

            shelf.addEventListener("pointermove", track as EventListener);
            shelf.addEventListener("pointerleave", rest);

            return () => {
              shelf.removeEventListener("pointermove", track as EventListener);
              shelf.removeEventListener("pointerleave", rest);
            };
          }
        }
      },
    );

    return () => {
      media.revert();
    };
  }, [root]);
}
