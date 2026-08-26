type BrandMarkProps = {
  size?: number;
  tone?: "full" | "accent" | "faint";
};

const FILL = {
  full: "var(--color-text-primary)",
  accent: "var(--color-accent)",
  faint: "var(--color-text-secondary)",
} as const;

/** The blade between two stars that share their tips */
export const MARK_BLADE =
  "M12 .1C12 8.192 15.648 12 23.4 12 15.648 12 12 15.808 12 23.9 12 15.808 8.352 12 .6 12 8.352 12 12 8.192 12 .1Z" +
  "M12 .1C12 10.215 13.71 12 23.4 12 13.71 12 12 13.785 12 23.9 12 13.785 10.29 12 .6 12 10.29 12 12 10.215 12 .1Z";

export const MARK_CORE =
  "M12 9.144C12 11.315 12.657 12 14.736 12c-2.079 0-2.736.685-2.736 2.856C12 12.685 11.343 12 9.264 12 11.343 12 12 11.315 12 9.144Z";

export function BrandMark({ size = 24, tone = "full" }: BrandMarkProps) {
  const fill = FILL[tone];

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      shapeRendering="geometricPrecision"
    >
      <path d={MARK_BLADE} fill={fill} fillRule="evenodd" />
      <path d={MARK_CORE} fill={fill} />
    </svg>
  );
}
