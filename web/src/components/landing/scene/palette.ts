import type { ScenePalette, SceneTheme } from "./types";

const DARK: ScenePalette = {
  background: 0x05070a,
  fog: 0x05070a,
  ink: "#eef5f7",
  quiet: "rgba(238,245,247,.42)",
  glass: 0xdfeaec,
  attenuation: 0xa8d4d8,
  rim: 0xe4eff1,
  keyLight: 0xf2f8fa,
  keyIntensity: 2.9,
  fillLight: 0x4d8f95,
  fillIntensity: 1.1,
  beam: 0xf6fbfc,
  beamIntensity: 5.2,
  hostKey: 0xfdffff,
  hostShadow: 0x39474f,
  exposure: 1.12,
  bloomStrength: 0.34,
  bloomThreshold: 0.95,
};

const LIGHT: ScenePalette = {
  background: 0xeef3f5,
  fog: 0xeef3f5,
  ink: "#0a1013",
  quiet: "rgba(10,16,19,.42)",
  glass: 0xc7d6da,
  attenuation: 0xcfe6e8,
  rim: 0x22323a,
  keyLight: 0xffffff,
  keyIntensity: 2.1,
  fillLight: 0x2f7a80,
  fillIntensity: 1.5,
  beam: 0x1d3238,
  beamIntensity: 2.1,
  hostKey: 0xffffff,
  hostShadow: 0x8fa0a8,
  exposure: 0.96,
  bloomStrength: 0.18,
  bloomThreshold: 1.04,
};

export function getPalette(theme: SceneTheme): ScenePalette {
  return theme === "dark" ? DARK : LIGHT;
}
