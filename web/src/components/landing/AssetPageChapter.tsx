"use client";

import gsap from "gsap";
import { Flip } from "gsap/Flip";
import { ScrollTrigger } from "gsap/ScrollTrigger";
import { useEffect, useRef, useState } from "react";
import { Shell } from "@/components/layout/Shell";
import styles from "./AssetPageChapter.module.css";

/** The elements the sheet lists for the block it is editing */
const SHEET_ELEMENTS = ["Description", "Personality", "First message"] as const;

export function AssetPageChapter() {
  const root = useRef<HTMLElement>(null);
  const [narrow, setNarrow] = useState(false);

  useWorkbench(root, setNarrow);

  return (
    <section
      className={styles.chapter}
      ref={root}
      aria-labelledby="landing-builder-title"
    >
      <Shell className={styles.stage}>
        <div className={styles.copy}>
          <h2 id="landing-builder-title" data-lift>
            Edit the page readers open.
          </h2>
          <p data-lift>
            Import a file or start from nothing. Choose the arrangement, edit
            the content, and publish from the same page a reader will open.
          </p>
        </div>

        <div className={styles.workbench} data-workbench aria-hidden="true">
          <div className={styles.preview}>
            <div className={styles.previewHead}>
              <span className={styles.previewTitle}>Your character</span>
              <span className={styles.previewMeta}>@yourhandle · Character</span>
            </div>

            <div className={styles.blocks}>
              <article className={styles.block}>
                <span>Description</span>
                <i />
                <i />
                <i data-short />
              </article>

              <article
                className={styles.block}
                data-subject
                data-narrow={narrow || undefined}
              >
                <span>Personality</span>
                <i />
                <i data-short />
              </article>

              <article
                className={styles.block}
                data-fills={narrow || undefined}
              >
                <span>First message</span>
                <i />
                <i data-short />
              </article>
            </div>
          </div>

          <aside className={styles.sheet} data-sheet>
            <header className={styles.sheetHead}>
              <strong>Personality</strong>
              <span>Block</span>
            </header>

            <div className={styles.sheetField}>
              Name
              <span>Personality</span>
            </div>

            <div className={styles.sheetRow}>
              Page arrangement
              <span>Beside the cover</span>
            </div>

            <div className={styles.sheetRow} data-width-row>
              Width
              <span data-width-value>{narrow ? "Half" : "Full"}</span>
            </div>

            <ul className={styles.sheetElements}>
              {SHEET_ELEMENTS.map((element) => (
                <li key={element}>{element}</li>
              ))}
            </ul>
          </aside>
        </div>
      </Shell>
    </section>
  );
}

/** Reveals the workbench and then changes the width of one block */
function useWorkbench(
  root: React.RefObject<HTMLElement | null>,
  setNarrow: (value: boolean) => void,
) {
  const played = useRef(false);

  useEffect(() => {
    const node = root.current;
    if (!node) return;

    gsap.registerPlugin(ScrollTrigger, Flip);
    const media = gsap.matchMedia();

    media.add(
      {
        motion: "(prefers-reduced-motion: no-preference)",
        twoColumn: "(min-width: 621px)",
      },
      (context) => {
        const { motion, twoColumn } = context.conditions as {
          motion: boolean;
          twoColumn: boolean;
        };
        if (!motion) return;

        const lifts = node.querySelectorAll("[data-lift]");
        const workbench = node.querySelector("[data-workbench]");
        const sheet = node.querySelector("[data-sheet]");
        const widthRow = node.querySelector("[data-width-row]");

        const reveal = gsap.timeline({
          scrollTrigger: { trigger: node, start: "top 66%", once: true },
        });

        reveal
          .from(lifts, {
            opacity: 0,
            y: 22,
            duration: 0.6,
            stagger: 0.1,
            ease: "power3.out",
          })
          .from(
            workbench,
            { opacity: 0, y: 40, duration: 0.8, ease: "power3.out" },
            0.05,
          )
          .from(
            sheet,
            { xPercent: 12, opacity: 0, duration: 0.7, ease: "power3.out" },
            0.34,
          )
          .call(
            () => {
              if (played.current || !twoColumn) return;
              played.current = true;
              performEdit(node, widthRow, setNarrow);
            },
            [],
            0.9,
          );
      },
    );

    return () => {
      media.revert();
    };
  }, [root, setNarrow]);
}

/** Marks the control and lets the preview reflow around the new width */
function performEdit(
  node: HTMLElement,
  widthRow: Element | null,
  setNarrow: (value: boolean) => void,
) {
  const blocks = node.querySelectorAll("[data-subject], [data-fills]");
  const subject = node.querySelector("[data-subject]");

  if (widthRow) {
    gsap.fromTo(
      widthRow,
      { backgroundColor: "rgba(128,128,128,0)" },
      {
        backgroundColor: "rgba(128,128,128,0.16)",
        duration: 0.24,
        repeat: 1,
        yoyo: true,
        ease: "power2.inOut",
      },
    );
  }

  window.setTimeout(() => {
    const state = Flip.getState(blocks);
    setNarrow(true);

    requestAnimationFrame(() => {
      Flip.from(state, {
        duration: 0.72,
        ease: "power3.inOut",
        absolute: true,
        onComplete: () => {
          if (subject) gsap.set(subject, { clearProps: "transform" });
        },
      });
    });
  }, 300);
}
