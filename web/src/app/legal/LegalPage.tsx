import Link from "next/link";
import type { ReactNode } from "react";
import { Shell } from "@/components/layout/Shell";
import {
  LEGAL_DOCUMENTS,
  LEGAL_EFFECTIVE_DATE,
  type LegalHref,
} from "@/lib/legal-documents";
import styles from "./LegalPage.module.css";

type LegalPageProps = {
  href: LegalHref;
  title: string;
  lede: ReactNode;
  children: ReactNode;
};

export function LegalPage({ href, title, lede, children }: LegalPageProps) {
  return (
    <div className={styles.page}>
      <Shell className={styles.column}>
        <article>
          <header className={styles.heading}>
            <p className={styles.kicker}>Legal</p>
            <h1>{title}</h1>
            <p className={styles.effective}>Effective {LEGAL_EFFECTIVE_DATE}</p>
          </header>

          <p className={styles.lede}>{lede}</p>

          <div className={styles.body}>{children}</div>
        </article>

        <nav className={styles.nav} aria-label="Legal documents">
          {LEGAL_DOCUMENTS.filter((document) => document.href !== href).map(
            (document) => (
              <Link key={document.href} href={document.href}>
                {document.title}
              </Link>
            ),
          )}
        </nav>
      </Shell>
    </div>
  );
}
