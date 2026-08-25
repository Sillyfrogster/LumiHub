"use client";

import gsap from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";
import { ArrowRight } from "lucide-react";
import { useEffect, useRef } from "react";
import { Shell } from "@/components/layout/Shell";
import { Button } from "@/components/ui/Button";
import styles from "./CloseChapter.module.css";

const CLOSING_LINES = [
  ["Find", "something"],
  ["to", "use."],
] as const;

export function CloseChapter() {
  const root = useRef<HTMLElement>(null);

  useCloseReveal(root);

  return (
    <section
      className={styles.chapter}
      ref={root}
      aria-labelledby="landing-close-title"
    >
      <Shell className={styles.stage}>
        <h2 id="landing-close-title" className={styles.statement}>
          {CLOSING_LINES.map((line) => (
            <span className={styles.line} key={line.join(" ")}>
              {line.map((word) => (
                <span className={styles.mask} key={word}>
                  <span className={styles.word} data-word>
                    {word}
                  </span>
                </span>
              ))}
            </span>
          ))}
        </h2>

        <div className={styles.action} data-action>
          <Button href="/browse" size="large">
            Browse the catalog
            <ArrowRight size={16} strokeWidth={1.5} aria-hidden="true" />
          </Button>
        </div>
      </Shell>
    </section>
  );
}

/** The field wipes open, then the closing line rises out of it */
function useCloseReveal(root: React.RefObject<HTMLElement | null>) {
  useEffect(() => {
    const node = root.current;
    if (!node) return;

    gsap.registerPlugin(ScrollTrigger);
    const media = gsap.matchMedia();

    media.add("(prefers-reduced-motion: no-preference)", () => {
      const words = node.querySelectorAll("[data-word]");
      const action = node.querySelector("[data-action]");

      const reveal = gsap.timeline({
        scrollTrigger: { trigger: node, start: "top 74%", once: true },
      });

      reveal
        .fromTo(
          node,
          { clipPath: "inset(42% 0% 42% 0%)" },
          {
            clipPath: "inset(0% 0% 0% 0%)",
            duration: 0.86,
            ease: "power3.out",
          },
          0,
        )
        .from(
          words,
          { yPercent: 116, duration: 0.62, stagger: 0.05, ease: "power3.out" },
          0.28,
        )
        .from(
          action,
          { opacity: 0, y: 20, duration: 0.5, ease: "power3.out" },
          0.62,
        );
    });

    return () => {
      media.revert();
    };
  }, [root]);
}
