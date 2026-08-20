import { describe, expect, test } from "bun:test";
import {
  applyThemePreference,
  readThemePreference,
  THEME_STORAGE_KEY,
} from "./theme";

function memoryStorage(initial?: string) {
  const values = new Map<string, string>();
  if (initial) values.set(THEME_STORAGE_KEY, initial);
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  };
}

function rootWith(theme?: string) {
  const dataset: { theme?: string } = { theme };
  return {
    dataset,
    removeAttribute(name: string) {
      if (name === "data-theme") delete dataset.theme;
    },
  };
}

describe("theme preference", () => {
  test("reads only explicit light and dark overrides", () => {
    expect(readThemePreference(memoryStorage("light"))).toBe("light");
    expect(readThemePreference(memoryStorage("dark"))).toBe("dark");
    expect(readThemePreference(memoryStorage("sepia"))).toBe("system");
    expect(readThemePreference(memoryStorage())).toBe("system");
  });

  test("stores an override and applies it to the root", () => {
    const storage = memoryStorage();
    const root = rootWith();

    applyThemePreference("dark", root, storage);

    expect(root.dataset.theme).toBe("dark");
    expect(storage.getItem(THEME_STORAGE_KEY)).toBe("dark");
  });

  test("system mode removes the override from the root and storage", () => {
    const storage = memoryStorage("dark");
    const root = rootWith("dark");

    applyThemePreference("system", root, storage);

    expect(root.dataset.theme).toBeUndefined();
    expect(storage.getItem(THEME_STORAGE_KEY)).toBeNull();
  });

  test("keeps working when storage is unavailable", () => {
    const storage = {
      getItem(): string | null {
        throw new Error("unavailable");
      },
      setItem(): void {
        throw new Error("unavailable");
      },
      removeItem(): void {
        throw new Error("unavailable");
      },
    };
    const root = rootWith();

    expect(readThemePreference(storage)).toBe("system");
    applyThemePreference("light", root, storage);
    expect(root.dataset.theme).toBe("light");
  });
});
