/** The four legal documents, in the order they are read. */
export const LEGAL_DOCUMENTS = [
  { href: "/legal/terms", title: "Terms of Service" },
  { href: "/legal/privacy", title: "Privacy Policy" },
  { href: "/legal/acceptable-use", title: "Acceptable Use" },
  { href: "/legal/dmca", title: "DMCA / Copyright" },
] as const;

export type LegalHref = (typeof LEGAL_DOCUMENTS)[number]["href"];

/** One date covers all four documents, because they were rewritten together. */
export const LEGAL_EFFECTIVE_DATE = "23 August 2026";
