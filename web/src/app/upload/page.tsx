import { Shell } from "@/components/layout/Shell";
import { pageMetadata } from "@/lib/site-metadata";
import { UploadFlow } from "./UploadFlow";
import styles from "./UploadPage.module.css";

export const metadata = pageMetadata(
  "Publish an asset",
  "Import a file you already have, or start a new character, lorebook, preset, theme or pack.",
);

export default function UploadPage() {
  return (
    <section className={styles.page}>
      <Shell className={styles.layout}>
        <div className={styles.desk}>
          <div className={styles.task}>
            <header className={styles.heading}>
              <h1>Publish or start an asset</h1>
              <p>
                Import an original file or open a new private draft in Illarin's
                builder.
              </p>
            </header>
            <UploadFlow />
          </div>
          <div className={styles.artwork} aria-hidden="true">
            <span className={styles.archiveHost} />
          </div>
        </div>
      </Shell>
    </section>
  );
}
