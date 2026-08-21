import Image from "next/image";
import archiveHost from "@/assets/art/full/illarin-mascot-archive-v1.webp";
import { Shell } from "@/components/layout/Shell";
import { UploadFlow } from "./UploadFlow";
import styles from "./UploadPage.module.css";

export default function UploadPage() {
  return (
    <section className={styles.page}>
      <Shell className={styles.layout}>
        <header className={styles.heading}>
          <div>
            <h1>Publish or start an asset</h1>
            <p>
              Import an original file or open a new private draft in Illarin's
              builder.
            </p>
          </div>
          <Image
            src={archiveHost}
            alt="Illarin's host carries a catalog folio"
            className={styles.mascot}
            sizes="260px"
            priority
          />
        </header>
        <UploadFlow />
      </Shell>
    </section>
  );
}
