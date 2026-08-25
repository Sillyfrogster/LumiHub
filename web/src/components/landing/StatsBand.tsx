import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import { SITE_STATS } from "@/data/site";
import styles from "./StatsBand.module.css";

export function StatsBand() {
  return (
    <section className={styles.band}>
      <Art name="wash" sizes="70vw" className={styles.washLeft} />
      <Art name="wash" sizes="60vw" className={styles.washRight} />

      <Shell>
        <ul className={styles.grid}>
          {SITE_STATS.map((stat) => (
            <li key={stat.label} className={styles.stat}>
              <p className={styles.value}>{stat.value}</p>
              <p className={styles.label}>{stat.label}</p>
            </li>
          ))}
        </ul>
      </Shell>
    </section>
  );
}
