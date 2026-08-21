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
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M4 21V11.5C4 6.8 7.4 3 12 3s8 3.8 8 8.5V21"
        stroke={FILL[tone]}
        strokeWidth="1.7"
      />
      <path
        d="M8 21V12.2C8 9.4 9.7 7.3 12 7.3s4 2.1 4 4.9V21"
        stroke={FILL[tone]}
        strokeWidth="1.2"
        opacity="0.72"
      />
      <path d="M12 10.8V21" stroke={FILL[tone]} strokeWidth="1.5" />
      <circle
        cx="12"
        cy="6.2"
        r="1.35"
        fill={tone === "full" ? "var(--color-brand-detail)" : FILL[tone]}
      />
    </svg>
  );
}
