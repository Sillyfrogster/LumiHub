export function ArtFilters() {
  return (
    <svg
      width="0"
      height="0"
      aria-hidden="true"
      focusable="false"
      style={{ position: "absolute" }}
    >
      <filter id="watercolor-dark-ink" colorInterpolationFilters="sRGB">
        <feFlood floodColor="var(--color-accent)" result="ink" />
        <feComposite in="ink" in2="SourceAlpha" operator="in" />
      </filter>
    </svg>
  );
}
