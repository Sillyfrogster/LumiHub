import { BookOpen, SlidersHorizontal, User } from "lucide-react";
import Link from "next/link";
import { Shell } from "@/components/layout/Shell";
import { Eyebrow } from "@/components/ui/SectionHead";
import { CREATION_TYPES } from "@/data/site";
import type { CreationKind } from "@/types/creation";
import styles from "./TypeShowcase.module.css";

const ICONS: Record<CreationKind, typeof User> = {
  character: User,
  lorebook: BookOpen,
  preset: SlidersHorizontal,
};

export function TypeShowcase() {
  return (
    <Shell as="section" className={styles.section}>
      <div className={styles.grid}>
        <div>
          <Eyebrow>Browse by type</Eyebrow>
          <h2 className={styles.title}>Three shapes of a story</h2>
          <p className={styles.body}>
            A character to speak, a lorebook to remember, a preset to set the
            tone. Upload once — LumiHub converts and delivers it to whichever
            frontend your reader is using.
          </p>
        </div>

        <ul className={styles.types}>
          {CREATION_TYPES.map((type) => {
            const Icon = ICONS[type.kind];

            return (
              <li key={type.kind}>
                <Link
                  href={`/browse?type=${type.kind}s`}
                  className={styles.type}
                >
                  <span className={styles.icon}>
                    <Icon size={26} strokeWidth={1.2} />
                  </span>
                  <span className={styles.name}>{type.name}</span>
                  <span className={styles.count}>{type.count} creations</span>
                </Link>
              </li>
            );
          })}
        </ul>
      </div>
    </Shell>
  );
}
