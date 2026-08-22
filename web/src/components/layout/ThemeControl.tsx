"use client";

import { SunMoon } from "lucide-react";
import { useEffect, useState } from "react";
import {
  applyThemePreference,
  readThemePreference,
  type ThemePreference,
} from "@/lib/theme";
import styles from "./ThemeControl.module.css";

const THEME_CHANGE_EVENT = "illarin:theme-preference";

export function ThemeControl() {
  const [preference, setPreference] = useState<ThemePreference>("system");

  useEffect(() => {
    function syncPreference(event?: Event) {
      if (event instanceof CustomEvent && event.detail) {
        setPreference(event.detail as ThemePreference);
        return;
      }
      setPreference(readThemePreference());
    }

    syncPreference();
    window.addEventListener("storage", syncPreference);
    window.addEventListener(THEME_CHANGE_EVENT, syncPreference);
    return () => {
      window.removeEventListener("storage", syncPreference);
      window.removeEventListener(THEME_CHANGE_EVENT, syncPreference);
    };
  }, []);

  function handleChange(nextPreference: ThemePreference) {
    setPreference(nextPreference);
    applyThemePreference(nextPreference);
    window.dispatchEvent(
      new CustomEvent<ThemePreference>(THEME_CHANGE_EVENT, {
        detail: nextPreference,
      }),
    );
  }

  return (
    <label className={styles.control}>
      <span className={styles.icon} aria-hidden="true">
        <SunMoon size={17} strokeWidth={1.7} />
      </span>
      <span className={styles.label}>Appearance</span>
      <select
        className={styles.select}
        aria-label="Theme"
        value={preference}
        onChange={(event) =>
          handleChange(event.currentTarget.value as ThemePreference)
        }
      >
        <option value="system">System</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
    </label>
  );
}
