import type { ScenePalette, SceneTheme } from "./types";

const DARK: ScenePalette = {
  background: 0x050505,
  fog: 0x050505,
  ink: "#f2f2ef",
  quiet: "rgba(242,242,239,.42)",
  glass: 0xd8d8d4,
  attenuation: 0xbcbcb6,
  rim: 0xeeeeea,
  keyLight: 0xf7f7f3,
  keyIntensity: 2.9,
  fillLight: 0x8b8b86,
  fillIntensity: 1.1,
  beam: 0xffffff,
  beamIntensity: 5.2,
  hostKey: 0xffffff,
  hostShadow: 0x383836,
  exposure: 1.12,
  bloomStrength: 0.26,
  bloomThreshold: 1.14,
};

const LIGHT: ScenePalette = {
  background: 0xf1f1ef,
  fog: 0xf1f1ef,
  ink: "#0c0c0b",
  quiet: "rgba(12,12,11,.42)",
  glass: 0xc8c8c2,
  attenuation: 0xe2e2dd,
  rim: 0x30302e,
  keyLight: 0xffffff,
  keyIntensity: 2.1,
  fillLight: 0x777773,
  fillIntensity: 1.5,
  beam: 0x333330,
  beamIntensity: 2.1,
  hostKey: 0xffffff,
  hostShadow: 0x92928d,
  exposure: 0.96,
  bloomStrength: 0.14,
  bloomThreshold: 1.16,
};

export function getPalette(theme: SceneTheme): ScenePalette {
  return theme === "dark" ? DARK : LIGHT;
}
