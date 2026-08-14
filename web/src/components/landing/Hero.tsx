import { ArrowRight, Feather } from "lucide-react";
import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import { Parallax } from "@/components/motion/Parallax";
import { Button } from "@/components/ui/Button";
import styles from "./Hero.module.css";

export function Hero() {
  return (
    <section className={styles.hero}>
      <Parallax speed={-0.035} className={styles.heroArt}>
        <Art
          name="hero-lumi"
          alt="Lumi, the guide of LumiHub"
          sizes="(max-width: 1100px) 100vw, 56vw"
          preload
        />
      </Parallax>

      <Parallax speed={-0.06} className={styles.wash}>
        <Art name="wash" width={520} preload />
      </Parallax>
      <Art name="birds" width={200} className={styles.birds} />
      <Art name="sparkles" width={230} className={styles.sparkles} />

      <Shell className={styles.grid}>
        <div className={styles.copy}>
          <h1 className={styles.title}>
            <span className={styles.line}>Find something</span>
            <span className={styles.line}>
              worth <em>remembering.</em>
            </span>
          </h1>

          <p className={styles.lede}>
            Characters, lorebooks, presets, and themes gathered in one quiet
            catalog, ready for wherever your next story unfolds.
          </p>

          <div className={styles.actions}>
            <Button href="/browse" size="large">
              Browse the collection
              <ArrowRight size={15} strokeWidth={1.4} />
            </Button>
            <Button href="/upload" variant="outline" size="large">
              <Feather size={15} strokeWidth={1.25} />
              Publish your work
            </Button>
          </div>

          <p className={styles.promise}>
            The file a creator made stays the source of truth.
          </p>
        </div>
      </Shell>
    </section>
  );
}
