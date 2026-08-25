import Link from "next/link";
import { Art } from "@/components/art/Art";
import { BrandMark } from "@/components/brand/BrandMark";
import { Shell } from "./Shell";
import styles from "./SiteFooter.module.css";

const LINKS = [
  { label: "About", href: "/about" },
  { label: "Creator Guide", href: "/guide" },
  { label: "API", href: "/docs" },
  { label: "Guidelines", href: "/guidelines" },
  { label: "Privacy", href: "/legal/privacy" },
  { label: "Terms", href: "/legal/terms" },
];

export function SiteFooter() {
  return (
    <footer className={styles.footer}>
      <Shell className={styles.inner}>
        <Art name="divider-star" width={340} className={styles.rule} />
        <p className={styles.tagline}>
          Made for creators. Inspired by stories. Built for every frontend.
        </p>

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

          <span className={styles.copyright}>© 2025 LumiHub</span>
        </div>
      </Shell>
    </footer>
  );
}
