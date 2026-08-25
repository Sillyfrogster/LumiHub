"use client";

import gsap from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";
import { ArrowRight, Cable, Download } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef } from "react";
import { Shell } from "@/components/layout/Shell";
import styles from "./DeliveryChapter.module.css";

type DeliveryChapterProps = {
  /** Applications the catalog actually carries work for */
  platforms: { label: string; value: string }[];
};

export function DeliveryChapter({ platforms }: DeliveryChapterProps) {
  const root = useRef<HTMLElement>(null);
  const destination = platforms[0]?.label ?? "a linked application";

  useDeliveryReveal(root);

  return (
    <section
      className={styles.chapter}
      ref={root}
      aria-labelledby="landing-delivery-title"
    >
      <Shell className={styles.stage}>
        <div className={styles.copy}>
          <h2 id="landing-delivery-title" data-lift>
            Download it. Or send it straight in.
          </h2>
          <p data-lift>
            Every asset offers the formats its content can actually carry. Take
            the file, or send it to an application you have linked and open it
            already there.
          </p>
          <Link href="/browse" className={styles.action} data-lift>
            Find something to use
            <ArrowRight size={16} strokeWidth={1.5} aria-hidden="true" />
          </Link>
        </div>

        <div className={styles.paths} aria-hidden="true">
          <ol className={styles.route} data-route>
            <li className={styles.stop}>
              <strong>Narcissa Roux</strong>
              <small>The work, as its creator uploaded it</small>
            </li>

            <li className={styles.wire} data-wire>
              <i data-token />
            </li>

            <li className={styles.stop} data-format>
              <strong>Character card</strong>
              <small>The format this content can carry</small>
            </li>

            <li className={styles.wire} data-wire>
              <i data-token />
            </li>

            <li className={styles.stop} data-destination>
              <Cable size={17} strokeWidth={1.4} aria-hidden="true" />
              <span>
                <strong>{destination}</strong>
                <small>Linked. It opens there, already in place.</small>
              </span>
            </li>
          </ol>

          <p className={styles.baseline} data-baseline>
            <Download size={15} strokeWidth={1.5} aria-hidden="true" />
            Or take the file and put it wherever you like.
          </p>
        </div>
      </Shell>
    </section>
  );
}

/** The asset travels the wire into the linked application, once, on reveal */
function useDeliveryReveal(root: React.RefObject<HTMLElement | null>) {
  useEffect(() => {
    const node = root.current;
    if (!node) return;

    gsap.registerPlugin(ScrollTrigger);
    const media = gsap.matchMedia();

    media.add("(prefers-reduced-motion: no-preference)", () => {
      const lifts = node.querySelectorAll("[data-lift]");
      const route = node.querySelector("[data-route]");
      const tokens = node.querySelectorAll("[data-token]");
      const destinationNode = node.querySelector("[data-destination]");
      const baseline = node.querySelector("[data-baseline]");

      const reveal = gsap.timeline({
        scrollTrigger: { trigger: node, start: "top 68%", once: true },
      });

      reveal
        .from(lifts, {
          opacity: 0,
          y: 22,
          duration: 0.6,
          stagger: 0.09,
          ease: "power3.out",
        })
        .from(
          route,
          { opacity: 0, y: 34, duration: 0.7, ease: "power3.out" },
          0.08,
        )
        .from(
          baseline,
          { opacity: 0, y: 16, duration: 0.5, ease: "power3.out" },
          0.42,
        )
        .fromTo(
          tokens,
          { yPercent: -30, opacity: 0 },
          {
            yPercent: 240,
            opacity: 1,
            duration: 0.62,
            stagger: 0.34,
            ease: "power2.inOut",
          },
          0.5,
        )
        .to(tokens, { opacity: 0, duration: 0.16 }, ">-0.12")
        .fromTo(
          destinationNode,
          { borderColor: "var(--color-border)" },
          {
            borderColor: "var(--color-emphasis-border)",
            duration: 0.3,
            ease: "power2.out",
          },
          ">-0.22",
        );
    });

    return () => {
      media.revert();
    };
  }, [root]);
}
