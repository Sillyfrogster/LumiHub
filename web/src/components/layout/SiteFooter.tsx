import Link from "next/link";
import { Art } from "@/components/art/Art";
import { BrandMark } from "@/components/brand/BrandMark";
import { Shell } from "./Shell";
import styles from "./SiteFooter.module.css";

const LINKS = [
  { label: "Browse", href: "/browse" },
  { label: "Sign in", href: "/sign-in" },
  { label: "Create account", href: "/sign-up" },
];

export function SiteFooter() {
  return (
    <footer className={styles.footer}>
      <Shell className={styles.inner}>
        <Art name="divider-star" width={340} className={styles.rule} />
        <p className={styles.tagline}>A quiet home for work made carefully.</p>

        <div className={styles.meta}>
          <div className={styles.brand}>
            <BrandMark size={18} tone="faint" />
            <span className={styles.wordmark}>LumiHub</span>
          </div>

          <nav className={styles.links}>
            {LINKS.map((link) => (
              <Link key={link.label} href={link.href}>
                {link.label}
              </Link>
            ))}
          </nav>

          <span className={styles.copyright}>© 2026 LumiHub</span>
        </div>
      </Shell>
    </footer>
  );
}
