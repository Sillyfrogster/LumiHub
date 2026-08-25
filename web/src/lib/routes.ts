import type { Creation } from "@/types/creation";

export function creationHref(creation: Creation): string {
  return `/${creation.kind}s/${creation.id}`;
}
