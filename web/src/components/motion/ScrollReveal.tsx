"use client";

import { useEffect } from "react";

/** Fades a page's [data-reveal] sections in as they reach the viewport */
export function ScrollReveal() {
  useEffect(() => {
    const root = document.documentElement;
    const targets = Array.from(
      document.querySelectorAll<HTMLElement>("[data-reveal]"),
    );

    root.setAttribute("data-reveal-ready", "");

    const reveal = (element: HTMLElement) => {
      element.setAttribute("data-revealed", "");
    };

    if (!("IntersectionObserver" in window)) {
      targets.forEach(reveal);
      return () => root.removeAttribute("data-reveal-ready");
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

    return () => {
      observer.disconnect();
      root.removeAttribute("data-reveal-ready");
    };
  }, []);

  return null;
}
