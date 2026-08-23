import { notFound } from "next/navigation";

/** A v1 surface Illarin does not carry. It is gone rather than moved, so nothing redirects. */
export function RemovedSurface(): never {
  notFound();
}
