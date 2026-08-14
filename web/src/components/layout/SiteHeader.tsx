"use client";

import { Menu, Upload, X } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { BrandMark } from "@/components/brand/BrandMark";
import { useAuth } from "@/lib/auth";
import { Shell } from "./Shell";
import styles from "./SiteHeader.module.css";

const NAV = [{ label: "Browse", href: "/browse" }];

export function SiteHeader() {
  const pathname = usePathname();
  const { account, signOut } = useAuth();
  const [signingOut, setSigningOut] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

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

        <button
          type="button"
          className={styles.menuToggle}
          aria-expanded={mobileOpen}
          aria-controls="mobile-navigation"
          onClick={() => setMobileOpen((open) => !open)}
        >
          {mobileOpen ? (
            <X size={20} aria-hidden="true" />
          ) : (
            <Menu size={20} aria-hidden="true" />
          )}
          <span className="sr-only">
            {mobileOpen ? "Close navigation" : "Open navigation"}
          </span>
        </button>
      </Shell>

      {mobileOpen ? (
        <nav id="mobile-navigation" className={styles.mobileNav}>
          <Shell className={styles.mobileNavInner}>
            {NAV.map((item) => (
              <Link
                key={item.label}
                href={item.href}
                onClick={() => setMobileOpen(false)}
              >
                {item.label}
              </Link>
            ))}
            <Link
              href={account?.emailVerified ? "/upload" : "/sign-up"}
              onClick={() => setMobileOpen(false)}
            >
              {account?.emailVerified ? "Publish an asset" : "Create account"}
            </Link>
          </Shell>
        </nav>
      ) : null}
    </header>
  );
}
