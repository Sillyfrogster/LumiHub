import { ArrowRight, Users } from "lucide-react";
import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import { Parallax } from "@/components/motion/Parallax";
import { Button } from "@/components/ui/Button";
import { Eyebrow } from "@/components/ui/SectionHead";
import { FRONTENDS } from "@/data/site";
import styles from "./Hero.module.css";

export function Hero() {
  return (
    <section className={styles.hero}>
      <Parallax speed={-0.035} className={styles.heroArt}>
        <Art
          name="hero-lumi"
          alt="Lumi, the guide of LumiHub"
          sizes="(max-width: 1100px) 100vw, 56vw"
          priority
        />
      </Parallax>

      <Parallax speed={-0.06} className={styles.wash}>
        <Art name="wash" width={520} />
      </Parallax>
      <Art name="birds" width={200} className={styles.birds} />
      <Art name="sparkles" width={230} className={styles.sparkles} />

      <Shell className={styles.grid}>
        <div className={styles.copy}>
          <Eyebrow className={styles.eyebrow}>
            The creative hub for storytellers
          </Eyebrow>

          <h1 className={styles.title}>
            <span className={styles.line}>Create for</span>
            <span className={styles.line}>
              <em>every</em> frontend.
            </span>
          </h1>

          <p className={styles.lede}>
            Share characters, lorebooks and presets across any roleplay or
            storytelling frontend. One hub. Endless stories.
          </p>

          <div className={styles.actions}>
            <Button href="/browse" size="large">
              Explore Creations
              <ArrowRight size={15} strokeWidth={1.4} />
            </Button>
            <Button href="/community" variant="outline" size="large">
              <Users size={15} strokeWidth={1.25} />
              Join the Community
            </Button>
          </div>

          <div className={styles.support}>
            <p className={styles.supportLabel}>Built for every frontend</p>
            <ul className={styles.frontends}>
              {FRONTENDS.map((frontend) => (
                <li key={frontend.name} className={styles.frontend}>
                  <span className={styles.mark}>{frontend.mark}</span>
                  {frontend.name}
                </li>
              ))}
              <li className={styles.more}>&amp; more</li>
            </ul>
          </div>
        </div>

        <aside className={styles.note}>
          <p className={styles.noteTitle}>
            Share
            <br />
            Inspire
            <br />
            Create
          </p>
          <hr className={styles.noteRule} />
          <p className={styles.noteBody}>
            A place for creators and storytellers to bring worlds to life and
            empower the next great adventure.
          </p>
          <Art name="sprig" width={52} className={styles.sprig} />
        </aside>
      </Shell>
    </section>
  );
}
