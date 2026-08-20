"use client";

import { SunMoon } from "lucide-react";
import { useEffect, useRef } from "react";
import {
  applyThemePreference,
  readThemePreference,
  type ThemePreference,
} from "@/lib/theme";
import styles from "./ThemeControl.module.css";

export function ThemeControl() {
  const selectRef = useRef<HTMLSelectElement>(null);

  useEffect(() => {
    if (selectRef.current) {
      selectRef.current.value = readThemePreference();
    }
  }, []);

  return (
    <label className={styles.control} title="Theme">
      <select
        ref={selectRef}
        className={styles.select}
        aria-label="Theme"
        defaultValue="system"
        onChange={(event) =>
          applyThemePreference(event.currentTarget.value as ThemePreference)
        }
      >
        <option value="system">Use system theme</option>
        <option value="light">Use light theme</option>
        <option value="dark">Use dark theme</option>
      </select>
      <span className={styles.icon} aria-hidden="true">
        <SunMoon size={18} strokeWidth={1.7} />
      </span>
    </label>
  );
}
