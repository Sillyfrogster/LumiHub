import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { Shell } from "@/components/layout/Shell";
import styles from "./CreatorSection.module.css";

const PROMISES = [
  {
    title: "The original stays original",
    body: "LumiHub stores the file you uploaded and keeps catalog writing beside it, not inside it.",
  },
  {
    title: "One work, one lasting link",
    body: "New revisions keep the same asset address, so a shared find does not disappear after an update.",
  },
  {
    title: "More than one destination",
    body: "Readers can take the plain file or use an export target the format supports.",
  },
] as const;

export function CreatorSection() {
  return (
    <Shell as="section" className={styles.section}>
      <div className={styles.grid}>
        <div className={styles.copy}>
          <h2>Your work stays your work.</h2>
          <p>
            Publish the catalog entry without letting the catalog rewrite the
            artifact. LumiHub keeps the creator's file at the center.
          </p>
          <Link href="/upload" className={styles.link}>
            Publish an asset
            <ArrowRight size={15} aria-hidden="true" />
          </Link>
        </div>

        <ol className={styles.promises}>
          {PROMISES.map((promise) => (
            <li key={promise.title}>
              <h3>{promise.title}</h3>
              <p>{promise.body}</p>
            </li>
          ))}
        </ol>
      </div>
    </Shell>
  );
}
