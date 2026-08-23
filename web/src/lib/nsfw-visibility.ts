import type { NsfwVisibility } from "@/lib/api/query";

const VISIBILITY_KEY = "illarin.nsfw-visibility";
const REVEAL_KEY = "illarin.nsfw-reveal:v1";

export function readSessionVisibility(): NsfwVisibility | undefined {
  try {
    const stored = window.sessionStorage.getItem(VISIBILITY_KEY);
    return isVisibility(stored) ? stored : undefined;
  } catch {
    return undefined;
  }
}

export function writeSessionVisibility(visibility: NsfwVisibility) {
  try {
    window.sessionStorage.setItem(VISIBILITY_KEY, visibility);
  } catch {}
}

export function readAssetReveal(id: string) {
  try {
    return window.sessionStorage.getItem(`${REVEAL_KEY}:${id}`) === "1";
  } catch {
    return false;
  }
}

export function writeAssetReveal(id: string) {
  try {
    window.sessionStorage.setItem(`${REVEAL_KEY}:${id}`, "1");
  } catch {}
}

function isVisibility(value: string | null): value is NsfwVisibility {
  return value === "hidden" || value === "blurred" || value === "shown";
}
