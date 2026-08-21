export type ThreeModule = typeof import("three");

export type SceneTheme = "dark" | "light";

export type SceneLayout = "portrait" | "tablet" | "wide";

export type PointerPosition = {
  x: number;
  y: number;
};

export type ScenePalette = {
  background: number;
  fog: number;
  ink: string;
  quiet: string;
  glass: number;
  attenuation: number;
  rim: number;
  keyLight: number;
  keyIntensity: number;
  fillLight: number;
  fillIntensity: number;
  beam: number;
  beamIntensity: number;
  hostKey: number;
  hostShadow: number;
  exposure: number;
  bloomStrength: number;
  bloomThreshold: number;
};
