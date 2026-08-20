type BrandMarkProps = {
  size?: number;
  tone?: "full" | "accent" | "faint";
};

const STAR =
  "M12 0 13.15 10.85 24 12 13.15 13.15 12 24 10.85 13.15 0 12 10.85 10.85Z";
const INNER_STAR =
  "M12 3.6 12.8 11.2 20.4 12 12.8 12.8 12 20.4 11.2 12.8 3.6 12 11.2 11.2Z";

const FILL = {
  full: "var(--color-text-primary)",
  accent: "var(--color-accent)",
  faint: "var(--color-text-secondary)",
} as const;

export function BrandMark({ size = 24, tone = "full" }: BrandMarkProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true">
      <path d={STAR} fill={FILL[tone]} />
      {tone === "full" && (
        <path
          d={INNER_STAR}
          fill="var(--color-accent)"
          opacity="0.55"
          transform="rotate(45 12 12)"
        />
      )}
    </svg>
  );
}
