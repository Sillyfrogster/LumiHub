type BrandMarkProps = {
  size?: number;
  tone?: "full" | "accent" | "faint";
};

const FILL = {
  full: "var(--color-text-primary)",
  accent: "var(--color-accent)",
  faint: "var(--color-text-secondary)",
} as const;

export function BrandMark({ size = 24, tone = "full" }: BrandMarkProps) {
  const fill = FILL[tone];
  const detail = tone === "full" ? "var(--color-brand-detail)" : fill;

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      shapeRendering="geometricPrecision"
    >
      <path
        d="M12 1.2 14.2 8.4 12 10.55 9.8 8.4 12 1.2Zm10.8 10.8-7.2 2.2L13.45 12l2.15-2.2 7.2 2.2ZM12 22.8l-2.2-7.2 2.2-2.15 2.2 2.15-2.2 7.2ZM1.2 12l7.2-2.2 2.15 2.2-2.15 2.2L1.2 12Z"
        fill={fill}
      />
      <path
        d="M12 2.85v4.8M21.15 12h-4.8M12 21.15v-4.8M2.85 12h4.8"
        stroke={detail}
        strokeLinecap="round"
        strokeWidth=".7"
        opacity={tone === "faint" ? 0.56 : 0.82}
      />
    </svg>
  );
}
