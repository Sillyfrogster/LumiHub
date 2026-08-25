import { ArrowRight } from "lucide-react";
import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import { Eyebrow } from "@/components/ui/SectionHead";
import styles from "./Newsletter.module.css";

export function Newsletter() {
  return (
    <section className={styles.section}>
      <Art name="balustrade" width={620} className={styles.balustrade} />

      <Shell className={styles.inner}>
        <div className={styles.copy}>
          <Eyebrow>Join the hub</Eyebrow>

          <h2 className={styles.title}>
            Stay inspired.
            <br />
            <em>Never miss a story.</em>
          </h2>

          <p className={styles.body}>
            Creations, creator spotlights and platform updates, straight to your
            inbox.
          </p>

          <form className={styles.form}>
            <label htmlFor="newsletter-email" className="sr-only">
              Email address
            </label>
            <input
              id="newsletter-email"
              type="email"
              placeholder="Enter your email"
              className={styles.input}
            />
            <button type="submit" className={styles.submit}>
              Subscribe
              <ArrowRight size={14} strokeWidth={1.4} />
            </button>
          </form>

          <p className={styles.fine}>No spam. Unsubscribe anytime.</p>
        </div>
      </Shell>
    </section>
  );
}
