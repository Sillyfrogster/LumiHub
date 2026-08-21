import { ArrowRight, Upload } from "lucide-react";
import Image from "next/image";
import thresholdArt from "@/assets/art/full/illarin-landing-threshold-v1.webp";
import { Shell } from "@/components/layout/Shell";
import { Button } from "@/components/ui/Button";
import styles from "./Hero.module.css";

export function Hero() {
  return (
    <section className={styles.hero}>
      <Image
        src={thresholdArt}
        alt=""
        fill
        priority
        sizes="100vw"
        className={styles.heroArt}
      />

      <Shell className={styles.grid}>
        <div className={styles.copy}>
          <h1 className={styles.title}>AI roleplay, in one catalog.</h1>

          <p className={styles.lede}>
            Find characters, lorebooks, presets, themes, and packs without
            chasing them across separate communities and applications.
          </p>

          <div className={styles.actions}>
            <Button href="/browse" size="large">
              Browse assets
              <ArrowRight size={15} strokeWidth={1.4} />
            </Button>
            <Button href="/upload" variant="outline" size="large">
              <Upload size={15} strokeWidth={1.4} />
              Publish
            </Button>
          </div>

          <p className={styles.promise}>
            Creator files stay intact. Catalog metadata and compatible exports
            stay beside them.
          </p>
        </div>
      </Shell>
    </section>
  );
}
