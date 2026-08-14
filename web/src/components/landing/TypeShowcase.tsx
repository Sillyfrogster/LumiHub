import {
  ArrowUpRight,
  BookOpen,
  Palette,
  SlidersHorizontal,
  User,
} from "lucide-react";
import Link from "next/link";
import { Shell } from "@/components/layout/Shell";
import type { BrowseKind } from "@/lib/api/query";
import styles from "./TypeShowcase.module.css";

const TYPES: Array<{
  kind: BrowseKind;
  name: string;
  description: string;
  icon: typeof User;
}> = [
  {
    kind: "character",
    name: "Characters",
    description: "Someone new to meet.",
    icon: User,
  },
  {
    kind: "lorebook",
    name: "Lorebooks",
    description: "A world with a memory.",
    icon: BookOpen,
  },
  {
    kind: "preset",
    name: "Presets",
    description: "The rhythm beneath a reply.",
    icon: SlidersHorizontal,
  },
  {
    kind: "theme",
    name: "Themes",
    description: "A different light for the room.",
    icon: Palette,
  },
];

export function TypeShowcase() {
  return (
    <Shell as="section" className={styles.section}>
      <div className={styles.grid}>
        <div className={styles.copy}>
          <h2 className={styles.title}>Every part of a story, together</h2>
          <p className={styles.body}>
            Browse one collection or narrow it to the part you need tonight.
            Kinds stay visible, so nothing has to hide behind a separate tab.
          </p>
        </div>

        <ul className={styles.types}>
          {TYPES.map((type) => {
            const Icon = type.icon;

            return (
              <li key={type.kind}>
                <Link
                  href={`/browse?kind=${type.kind}`}
                  className={styles.type}
                >
                  <span className={styles.icon}>
                    <Icon size={22} strokeWidth={1.25} aria-hidden="true" />
                  </span>
                  <span>
                    <span className={styles.name}>{type.name}</span>
                    <span className={styles.description}>
                      {type.description}
                    </span>
                  </span>
                  <ArrowUpRight
                    className={styles.arrow}
                    size={17}
                    aria-hidden="true"
                  />
                </Link>
              </li>
            );
          })}
        </ul>
      </div>
    </Shell>
  );
}
