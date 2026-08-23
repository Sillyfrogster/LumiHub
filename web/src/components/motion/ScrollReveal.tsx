"use client";

import { useEffect } from "react";

/**
 * Reveals [data-reveal] sections as they enter the viewport by adding
 * [data-revealed]. Purely additive: it touches nothing else on the page,
 * and readers with reduced motion never see hidden content because the
 * CSS keeps those elements visible without this script running.
 */
export function ScrollReveal() {
  useEffect(() => {
    const targets = Array.from(
      document.querySelectorAll<HTMLElement>("[data-reveal]"),
    );
    if (targets.length === 0) return;

    // Announce that revealing is active, so the CSS only hides sections
    // when this script is actually running.
    document.documentElement.setAttribute("data-reveal-ready", "");

    const reveal = (element: HTMLElement) => {
      element.setAttribute("data-revealed", "");
    };

    if (!("IntersectionObserver" in window)) {
      targets.forEach(reveal);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          reveal(entry.target as HTMLElement);
          observer.unobserve(entry.target);
        }
      },
      { threshold: 0.12, rootMargin: "0px 0px -8% 0px" },
    );

    for (const element of targets) observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return null;
}
