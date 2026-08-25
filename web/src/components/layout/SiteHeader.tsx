"use client";

import { Bell, ChevronDown, Search, Upload } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandMark } from "@/components/brand/BrandMark";
import { Shell } from "./Shell";
import styles from "./SiteHeader.module.css";

const NAV = [
  { label: "Discover", href: "/browse" },
  { label: "Characters", href: "/browse?type=characters" },
  { label: "Lorebooks", href: "/browse?type=lorebooks" },
  { label: "Presets", href: "/browse?type=presets" },
];

export function SiteHeader() {
  const pathname = usePathname();

  return (
    <header className={styles.header}>
      <Shell className={styles.bar}>
        <Link href="/" className={styles.brand}>
          <BrandMark />
          <span className={styles.wordmark}>LumiHub</span>
        </Link>

        <nav className={styles.nav}>
          {NAV.map((item) => (
            <Link
              key={item.label}
              href={item.href}
              className={styles.navLink}
              aria-current={pathname === item.href ? "page" : undefined}
            >
              {item.label}
            </Link>
          ))}
          <button type="button" className={styles.navLink}>
            Resources
            <ChevronDown size={12} strokeWidth={1.3} />
          </button>
        </nav>

        <div className={styles.spacer} />

        <button type="button" className={styles.search}>
          <Search size={14} strokeWidth={1.3} />
          <span className={styles.searchLabel}>Search LumiHub</span>
          <kbd className={styles.kbd}>⌘K</kbd>
        </button>

        <Link href="/upload" className={styles.upload}>
          <Upload size={13} strokeWidth={1.6} />
          Upload
        </Link>

        <button
          type="button"
          className={styles.iconButton}
          aria-label="Notifications"
        >
          <Bell size={17} strokeWidth={1.25} />
        </button>

        <div className={styles.avatar} />
      </Shell>
    </header>
  );
}
