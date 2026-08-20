import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import { UploadFlow } from "./UploadFlow";
import styles from "./UploadPage.module.css";

export default function UploadPage() {
  return (
    <section className={styles.page}>
      <Art name="wash" width={760} className={styles.wash} />
      <Art name="sprig" width={270} className={styles.sprig} loading="eager" />
      <Shell className={styles.layout}>
        <header className={styles.heading}>
          <h1>Bring your creation into the library</h1>
          <p>
            Bring in the original file for a creation you want to keep shaping.
          </p>
        </header>
        <UploadFlow />
      </Shell>
    </section>
  );
}
