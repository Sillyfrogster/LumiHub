"use client";

import { useEffect, useState } from "react";
import { LandingCanvas } from "@/components/landing/LandingCanvas";

/**
 * Development harness for the landing scene. It renders the canvas alone at
 * full bleed so a keyframe can be inspected without the page around it.
 *
 * Drive it from the console or a test runner:
 *   window.__scene.progress(0.42)
 *   window.__scene.theme("light")
 */
declare global {
  interface Window {
    __scene?: {
      progress: (value: number) => void;
      theme: (value: "dark" | "light") => void;
    };
  }
}

export default function LandingLab() {
  const [progress, setProgress] = useState(0);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    window.__scene = {
      progress: (value) => setProgress(Math.min(1, Math.max(0, value))),
      theme: (value) => {
        document.documentElement.dataset.theme = value;
      },
    };
    return () => {
      window.__scene = undefined;
    };
  }, []);

  return (
    <main
      style={{
        background: "var(--color-bg)",
        height: "100svh",
        position: "relative",
        width: "100vw",
      }}
    >
      <LandingCanvas
        progress={progress}
        enabled={!failed}
        onFailure={() => setFailed(true)}
      />
      <output
        style={{
          background: "rgba(0,0,0,.6)",
          borderRadius: 6,
          color: "#fff",
          fontFamily: "monospace",
          fontSize: 12,
          left: 12,
          padding: "6px 10px",
          position: "absolute",
          top: 12,
          zIndex: 20,
        }}
      >
        p={progress.toFixed(3)}
        {failed ? " · scene failed" : ""}
      </output>
    </main>
  );
}
