const PRESERVED_LABELS: Record<string, string> = {
  card: "unread character-card details",
  character_book: "extra lorebook details",
  chub: "Chub metadata",
  tavern_helper: "TavernHelper scripts",
  risuai: "RisuAI details",
  lumiverse_modules: "Lumiverse modules",
  landing_perspective_layers: "perspective layers",
  regex_scripts: "regex scripts",
};

export function describePreservedNamespace(namespace: string): string {
  return PRESERVED_LABELS[namespace] ?? "other format-specific details";
}

export function describePreservedNamespaces(namespaces: readonly string[]) {
  const labels = [
    ...new Set(
      namespaces.map((namespace) => describePreservedNamespace(namespace)),
    ),
  ];

  if (labels.length === 1) return labels[0];
  if (labels.length === 2) return labels.join(" and ");
  return `${labels.slice(0, -1).join(", ")}, and ${labels.at(-1)}`;
}
