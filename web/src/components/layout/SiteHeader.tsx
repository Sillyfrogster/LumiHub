"use client";

import { CircleUserRound, Menu, Upload, X } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { BrandMark } from "@/components/brand/BrandMark";
import { useAuth } from "@/lib/auth";
import { Shell } from "./Shell";
import styles from "./SiteHeader.module.css";
import { ThemeControl } from "./ThemeControl";

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
          <span className={styles.wordmark}>Illarin</span>
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

        <Link
          href={account?.emailVerified ? "/upload" : "/sign-up"}
          className={styles.publish}
        >
          <Upload size={13} strokeWidth={1.6} />
          Publish
        </Link>

        <details className={styles.account}>
          <summary className={styles.accountTrigger}>
            <CircleUserRound size={18} strokeWidth={1.55} aria-hidden="true" />
            <span>
              {account === undefined
                ? "Account"
                : account
                  ? `@${account.handle}`
                  : "Account"}
            </span>
          </summary>
          <div className={styles.accountMenu} aria-live="polite">
            {account ? (
              <>
                <div className={styles.accountIdentity}>
                  <strong>@{account.handle}</strong>
                  <span>
                    {account.emailVerified
                      ? "Verified account"
                      : "Email verification needed to publish"}
                  </span>
                </div>
                <Link href={`/@${account.handle}`}>View profile</Link>
                <Link href="/settings">Account settings</Link>
                {!account.emailVerified ? (
                  <Link href="/verify-email">Verify email</Link>
                ) : null}
              </>
            ) : (
              <>
                <Link href="/sign-in">Sign in</Link>
                <Link href="/sign-up">Create account</Link>
              </>
            )}
            <ThemeControl />
            {account ? (
              <button
                type="button"
                className={styles.signOut}
                onClick={handleSignOut}
                disabled={signingOut}
              >
                {signingOut ? "Signing out…" : "Sign out"}
              </button>
            ) : null}
          </div>
        </details>

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
            {account ? (
              <>
                <Link
                  href={`/@${account.handle}`}
                  onClick={() => setMobileOpen(false)}
                >
                  View profile
                </Link>
                <Link href="/settings" onClick={() => setMobileOpen(false)}>
                  Account settings
                </Link>
              </>
            ) : (
              <Link href="/sign-in" onClick={() => setMobileOpen(false)}>
                Sign in
              </Link>
            )}
            <ThemeControl />
            {account ? (
              <button
                className={styles.mobileSignOut}
                type="button"
                onClick={() => void handleSignOut()}
                disabled={signingOut}
              >
                {signingOut ? "Leaving…" : "Sign out"}
              </button>
            ) : null}
          </Shell>
        </nav>
      ) : null}
    </header>
  );
}
