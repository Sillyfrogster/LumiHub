"use client";

import { type ReactNode, useEffect, useRef } from "react";

type ParallaxProps = {
  speed: number;
  className?: string;
  children: ReactNode;
};

/** Drifts its contents against the scroll position */
export function Parallax({ speed, className, children }: ParallaxProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const element = ref.current;
    if (!element) {
      return;
    }

    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      return;
    }

    const onScroll = () => {
      element.style.transform = `translateY(${window.scrollY * speed}px)`;
    };

    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });

    return () => window.removeEventListener("scroll", onScroll);
  }, [speed]);

  return (
    <div ref={ref} className={className}>
      {children}
    </div>
  );
}
