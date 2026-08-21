"use client";

import { useEffect, useRef, useState } from "react";
import cardAtlas from "@/assets/art/cards/illarin-card-atlas-v1.png";
import hostArt from "@/assets/art/full/illarin-host-beam-v1.webp";
import hostDepth from "@/assets/art/full/illarin-host-beam-v1-depth.png";
import styles from "./LandingCanvas.module.css";
import type { LandingSceneTheme, LivingCatalogScene } from "./landing-scene";

type LandingCanvasProps = {
  progress: number;
  enabled: boolean;
  onFailure: () => void;
};

const clamp = (value: number, minimum = 0, maximum = 1) =>
  Math.min(maximum, Math.max(minimum, value));

function getTheme(systemTheme: MediaQueryList): LandingSceneTheme {
  const explicitTheme = document.documentElement.dataset.theme;
  if (explicitTheme === "dark" || explicitTheme === "light") {
    return explicitTheme;
  }
  return systemTheme.matches ? "dark" : "light";
}

/**
 * Depth of field costs a second full-resolution pass. Skip it where that is
 * likely to hurt more than it helps.
 */
function allowDepthOfField() {
  if (navigator.hardwareConcurrency && navigator.hardwareConcurrency <= 4) {
    return false;
  }
  return window.matchMedia("(min-width: 1080px)").matches;
}

export function LandingCanvas({
  progress,
  enabled,
  onFailure,
}: LandingCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const hostRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<LivingCatalogScene | null>(null);
  const progressRef = useRef(clamp(progress));
  const onFailureRef = useRef(onFailure);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    onFailureRef.current = onFailure;
  }, [onFailure]);

  useEffect(() => {
    progressRef.current = clamp(progress);
    sceneRef.current?.setProgress(progressRef.current);
  }, [progress]);

  useEffect(() => {
    if (!enabled) return;

    const canvas = canvasRef.current;
    const host = hostRef.current;
    if (!canvas || !host) return;

    let cancelled = false;
    let failed = false;
    let onScreen = false;
    let scene: LivingCatalogScene | null = null;

    const systemTheme = window.matchMedia("(prefers-color-scheme: dark)");
    const motionPreference = window.matchMedia(
      "(prefers-reduced-motion: reduce)",
    );

    const reportFailure = () => {
      if (failed || cancelled) return;
      failed = true;
      setReady(false);
      onFailureRef.current();
    };

    /** The loop runs only while the stage is on screen and the tab is shown. */
    const syncPlayback = () => {
      if (!scene || cancelled || failed) return;
      if (onScreen && !document.hidden) scene.start();
      else scene.stop();
    };

    const handleThemeChange = () => scene?.setTheme(getTheme(systemTheme));

    const handleMotionChange = () => {
      scene?.setReducedMotion(motionPreference.matches);
      syncPlayback();
    };

    const handlePointerMove = (event: PointerEvent) => {
      const bounds = host.getBoundingClientRect();
      if (bounds.width === 0 || bounds.height === 0) return;
      scene?.setPointer({
        x: clamp(((event.clientX - bounds.left) / bounds.width) * 2 - 1, -1, 1),
        y: clamp(((event.clientY - bounds.top) / bounds.height) * 2 - 1, -1, 1),
      });
    };

    const handlePointerLeave = () => scene?.setPointer({ x: 0, y: 0 });

    const handleContextLost = (event: Event) => {
      event.preventDefault();
      reportFailure();
    };

    const intersectionObserver = new IntersectionObserver(
      ([entry]) => {
        onScreen = Boolean(entry?.isIntersecting);
        syncPlayback();
      },
      { rootMargin: "120px 0px" },
    );
    intersectionObserver.observe(host);

    const resizeObserver = new ResizeObserver(() => {
      const bounds = host.getBoundingClientRect();
      scene?.resize(bounds.width, bounds.height);
    });
    resizeObserver.observe(host);

    const themeObserver = new MutationObserver(handleThemeChange);
    themeObserver.observe(document.documentElement, {
      attributeFilter: ["data-theme"],
      attributes: true,
    });

    host.addEventListener("pointermove", handlePointerMove, { passive: true });
    host.addEventListener("pointerleave", handlePointerLeave, {
      passive: true,
    });
    canvas.addEventListener("webglcontextlost", handleContextLost);
    document.addEventListener("visibilitychange", syncPlayback);
    motionPreference.addEventListener("change", handleMotionChange);
    systemTheme.addEventListener("change", handleThemeChange);

    Promise.all([import("three"), import("./landing-scene")])
      .then(([THREE, sceneModule]) => {
        if (cancelled) return;

        scene = sceneModule.createLivingCatalogScene(
          THREE,
          canvas,
          getTheme(systemTheme),
          {
            atlasUrl: cardAtlas.src,
            depthOfField: allowDepthOfField(),
            hostDepthUrl: hostDepth.src,
            hostUrl: hostArt.src,
            onAtlasFailure: reportFailure,
          },
        );
        sceneRef.current = scene;

        const bounds = host.getBoundingClientRect();
        scene.resize(bounds.width, bounds.height);
        scene.setReducedMotion(motionPreference.matches);
        scene.setProgress(progressRef.current);
        syncPlayback();
        setReady(true);
      })
      .catch(reportFailure);

    return () => {
      cancelled = true;
      setReady(false);

      intersectionObserver.disconnect();
      resizeObserver.disconnect();
      themeObserver.disconnect();
      host.removeEventListener("pointermove", handlePointerMove);
      host.removeEventListener("pointerleave", handlePointerLeave);
      canvas.removeEventListener("webglcontextlost", handleContextLost);
      document.removeEventListener("visibilitychange", syncPlayback);
      motionPreference.removeEventListener("change", handleMotionChange);
      systemTheme.removeEventListener("change", handleThemeChange);

      scene?.dispose();
      sceneRef.current = null;
    };
  }, [enabled]);

  return (
    <div
      ref={hostRef}
      aria-hidden="true"
      className={styles.root}
      data-enabled={enabled}
      data-ready={ready}
    >
      {/* biome-ignore lint/a11y/noAriaHiddenOnFocusable: This canvas is decorative and has no keyboard interaction. */}
      <canvas ref={canvasRef} aria-hidden="true" className={styles.canvas} />
    </div>
  );
}
