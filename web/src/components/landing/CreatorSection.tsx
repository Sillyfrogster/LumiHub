import { ArrowRight } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import mascotHost from "@/assets/art/full/illarin-mascot-host-v1.webp";
import { Shell } from "@/components/layout/Shell";
import styles from "./CreatorSection.module.css";

export function CreatorSection() {
  return (
    <section className={styles.section}>
      <Shell className={styles.grid}>
        <div className={styles.copy}>
          <h2>One source. Every compatible export.</h2>
          <p>
            Illarin stores the file a creator supplied. Descriptions, tags,
            revisions, and application-specific exports are managed around it
            instead of being written back into it.
          </p>
          <ol className={styles.route}>
            <li>
              <small>Source</small>
              <strong>Original creator file</strong>
            </li>
            <li>
              <small>Catalog</small>
              <strong>Metadata beside the work</strong>
            </li>
            <li>
              <small>Use</small>
              <strong>Original or compatible export</strong>
            </li>
          </ol>
          <Link href="/upload" className={styles.link}>
            Publish an asset
            <ArrowRight size={15} aria-hidden="true" />
          </Link>
        </div>
        <div className={styles.mascot}>
          <Image
            src={mascotHost}
            alt="Illarin's host gestures toward the publishing flow"
            sizes="(max-width: 780px) 72vw, 430px"
          />
        </div>
      </Shell>
    </section>
  );
}
