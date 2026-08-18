import type { components } from "@/lib/api/schema";

export type Scope = components["schemas"]["Scope"];

export type ScopeCopy = {
  /** What the instance may do, as a creator would say it. */
  title: string;
  /** The limit that makes granting it reasonable. */
  detail: string;
};

const SCOPES: Record<Scope, ScopeCopy> = {
  "asset:receive": {
    title: "Receive assets you send it",
    detail:
      "You choose what goes across. It cannot browse or take anything on its own.",
  },
  "library:sync": {
    title: "Report what it has installed",
    detail:
      "So LumiHub can show what you already have, and when a newer version exists.",
  },
};

/**
 * Describes one scope. An unrecognised scope is shown as it was named rather
 * than hidden, so a creator is never asked to approve something unexplained.
 */
export function describeScope(scope: Scope): ScopeCopy {
  return (
    SCOPES[scope] ?? {
      title: scope,
      detail: "This version of LumiHub does not recognise this permission.",
    }
  );
}
