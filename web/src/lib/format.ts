/** Shortens large counts, so 1600 reads as 1.6K */
export function formatCount(value: number): string {
  if (value < 1000) {
    return String(value);
  }

  const thousands = value / 1000;
  const rounded = thousands < 10 ? thousands.toFixed(1) : Math.round(thousands);

  return `${String(rounded).replace(/\.0$/, "")}K`;
}
