/** Timing helpers that behave the same on a 60Hz and a 144Hz display. */

export const clamp = (value: number, minimum = 0, maximum = 1) =>
  Math.min(maximum, Math.max(minimum, value));

export const mix = (from: number, to: number, amount: number) =>
  from + (to - from) * amount;

/** Chases a target. Smaller smoothing values close the gap faster. */
export const damp = (
  current: number,
  target: number,
  smoothing: number,
  delta: number,
) => target + (current - target) * Math.exp(-delta / Math.max(smoothing, 1e-4));

/** A slow drift that does not visibly repeat. */
export const drift = (time: number, speed: number, phase: number) =>
  Math.sin(time * speed + phase) * 0.6 +
  Math.sin(time * speed * 0.43 + phase * 1.7) * 0.3 +
  Math.sin(time * speed * 0.19 + phase * 2.3) * 0.1;
