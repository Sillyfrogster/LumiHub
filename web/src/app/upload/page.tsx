import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import { UploadFlow } from "./UploadFlow";
import styles from "./UploadPage.module.css";

export default function UploadPage() {
  return (
    <section className={styles.page}>
      <Art name="wash" width={760} className={styles.wash} loading="eager" />
      <Art name="sprig" width={270} className={styles.sprig} />
      <Shell className={styles.layout}>
        <header className={styles.heading}>
          <h1>Bring your creation into the library</h1>
          <p>
            Choose the original file and check how its catalog entry will read.
            LumiHub keeps those bytes untouched while it works out what they
            are.
          </p>
        </header>
        <UploadFlow />
      </Shell>
    </section>
  );
}
