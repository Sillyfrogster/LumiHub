"use client";

import { ChevronDown, Upload } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { BrandMark } from "@/components/brand/BrandMark";
import { useAuth } from "@/lib/auth";
import { Shell } from "./Shell";
import styles from "./SiteHeader.module.css";

const NAV = [
  { label: "Discover", href: "/browse" },
  { label: "Characters", href: "/browse?kind=character" },
  { label: "Lorebooks", href: "/browse?kind=lorebook" },
  { label: "Presets", href: "/browse?kind=preset" },
];

export function SiteHeader() {
  const pathname = usePathname();
  const { account, signOut } = useAuth();
  const [signingOut, setSigningOut] = useState(false);

  async function handleSignOut() {
    setSigningOut(true);
    try {
      await signOut();
    } finally {
      setSigningOut(false);
    }
  }

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

        {account?.emailVerified ? (
          <Link href="/upload" className={styles.upload}>
            <Upload size={13} strokeWidth={1.6} />
            Upload
          </Link>
        ) : null}

        <div className={styles.account} aria-live="polite">
          {account === undefined ? (
            <span className={styles.accountPending}>Reading session</span>
          ) : account ? (
            <>
              <span className={styles.identity}>
                <Link
                  href="/settings"
                  className={styles.handle}
                  aria-label={`Account settings for @${account.handle}`}
                >
                  @{account.handle}
                </Link>
                {!account.emailVerified ? (
                  <Link href="/verify-email" className={styles.unverified}>
                    Verify email
                  </Link>
                ) : null}
              </span>
              <button
                type="button"
                className={styles.signOut}
                onClick={handleSignOut}
                disabled={signingOut}
              >
                {signingOut ? "Leaving…" : "Sign out"}
              </button>
            </>
          ) : (
            <>
              <Link href="/sign-in" className={styles.signIn}>
                Sign in
              </Link>
              <Link href="/sign-up" className={styles.createAccount}>
                Create account
              </Link>
            </>
          )}
        </div>
      </Shell>
    </header>
  );
}
