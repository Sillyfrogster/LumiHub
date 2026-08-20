export const THEME_STORAGE_KEY = "theme-preference:v1";

export const THEME_PREFERENCES = ["system", "light", "dark"] as const;

export type ThemePreference = (typeof THEME_PREFERENCES)[number];
type ThemeRoot = Pick<HTMLElement, "dataset" | "removeAttribute">;
type ThemeStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export const THEME_BOOTSTRAP_SCRIPT = `try{var t=localStorage.getItem("${THEME_STORAGE_KEY}");if(t==="light"||t==="dark")document.documentElement.dataset.theme=t}catch{}`;

export function readThemePreference(storage?: ThemeStorage): ThemePreference {
  try {
    const preference = (storage ?? localStorage).getItem(THEME_STORAGE_KEY);
    return preference === "light" || preference === "dark"
      ? preference
      : "system";
  } catch {
    return "system";
  }
}

export function applyThemePreference(
  preference: ThemePreference,
  root: ThemeRoot = document.documentElement,
  storage?: ThemeStorage,
) {
  if (preference === "system") {
    root.removeAttribute("data-theme");
  } else {
    root.dataset.theme = preference;
  }

  try {
    const destination = storage ?? localStorage;
    if (preference === "system") {
      destination.removeItem(THEME_STORAGE_KEY);
    } else {
      destination.setItem(THEME_STORAGE_KEY, preference);
    }
  } catch {}
}
