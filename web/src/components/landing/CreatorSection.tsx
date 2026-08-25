import { Sparkles, Upload, Users } from "lucide-react";
import Link from "next/link";
import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import { Eyebrow } from "@/components/ui/SectionHead";
import { CREATOR_BENEFITS, SPOTLIGHT } from "@/data/site";
import styles from "./CreatorSection.module.css";

const ICONS = {
  upload: Upload,
  audience: Users,
  reward: Sparkles,
} as const;

export function CreatorSection() {
  return (
    <Shell as="section" className={styles.section}>
      <div className={styles.grid}>
        <div>
          <Eyebrow>Built for creators</Eyebrow>

          <ul className={styles.benefits}>
            {CREATOR_BENEFITS.map((benefit) => {
              const Icon = ICONS[benefit.icon];

              return (
                <li key={benefit.title}>
                  <span className={styles.icon}>
                    <Icon size={24} strokeWidth={1.2} />
                  </span>
                  <h3 className={styles.benefitTitle}>{benefit.title}</h3>
                  <p className={styles.benefitBody}>{benefit.body}</p>
                </li>
              );
            })}
          </ul>

          <Link href="/guide" className={styles.link}>
            Learn more about creator benefits →
          </Link>
        </div>

        <figure className={styles.spotlight}>
          <Art name="corner-tl" width={280} className={styles.corner} />
          <Eyebrow>Community spotlight</Eyebrow>
          <blockquote className={styles.quote}>“{SPOTLIGHT.quote}”</blockquote>

          <figcaption className={styles.person}>
            <span className={styles.avatar} />
            <span>
              <span className={styles.name}>{SPOTLIGHT.name}</span>
              <span className={styles.role}>{SPOTLIGHT.role}</span>
            </span>
          </figcaption>
        </figure>
      </div>
    </Shell>
  );
}
