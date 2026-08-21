import Link from "next/link";
import { BrandMark } from "@/components/brand/BrandMark";
import { Shell } from "./Shell";
import styles from "./SiteFooter.module.css";

const LINKS = [
  { label: "Browse", href: "/browse" },
  { label: "Publish", href: "/upload" },
  { label: "Account", href: "/settings" },
];

export function SiteFooter() {
  return (
    <footer className={styles.footer}>
      <Shell className={styles.inner}>
        <div className={styles.meta}>
          <div className={styles.brand}>
            <BrandMark size={18} tone="faint" />
            <span className={styles.wordmark}>Illarin</span>
          </div>

          <nav className={styles.links}>
            {LINKS.map((link) => (
              <Link key={link.label} href={link.href}>
                {link.label}
              </Link>
            ))}
          </nav>

          <p className={styles.description}>
            A cross-application catalog for AI roleplay assets.
          </p>

          <span className={styles.copyright}>© 2026 Illarin</span>
        </div>
      </Shell>
    </footer>
  );
}
