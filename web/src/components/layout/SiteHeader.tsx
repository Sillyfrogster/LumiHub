"use client";

import { CircleUserRound, Menu, Upload, X } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { BrandMark } from "@/components/brand/BrandMark";
import { useAuth } from "@/lib/auth";
import { Shell } from "./Shell";
import styles from "./SiteHeader.module.css";
import { ThemeControl } from "./ThemeControl";

const NAV = [{ label: "Browse", href: "/browse" }];
const UPLOAD_RETURN = encodeURIComponent("/upload");

function isCurrentPage(pathname: string, href: string) {
  const [path] = href.split("?");
  return pathname === path || pathname.startsWith(`${path}/`);
}

export function SiteHeader() {
  const pathname = usePathname();
  const { account, signOut } = useAuth();
  const [signingOut, setSigningOut] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const headerRef = useRef<HTMLElement>(null);
  const accountRef = useRef<HTMLDetailsElement>(null);
  const accountTriggerRef = useRef<HTMLElement>(null);
  const menuToggleRef = useRef<HTMLButtonElement>(null);
  const mobileNavRef = useRef<HTMLElement>(null);
  const previousPathname = useRef(pathname);

  const publishHref =
    account === undefined
      ? "/upload"
      : account === null
        ? `/sign-in?returnTo=${UPLOAD_RETURN}`
        : account.emailVerified
          ? "/upload"
          : `/verify-email?returnTo=${UPLOAD_RETURN}`;
  const publishLabel =
    account === undefined || account?.emailVerified
      ? "Publish"
      : account
        ? "Verify to publish"
        : "Sign in to publish";

  useEffect(() => {
    if (!mobileOpen) return;

    const animationFrame = window.requestAnimationFrame(() => {
      mobileNavRef.current
        ?.querySelector<HTMLElement>("a[href], select, button:not([disabled])")
        ?.focus();
    });

    return () => window.cancelAnimationFrame(animationFrame);
  }, [mobileOpen]);

  useEffect(() => {
    function handlePointerDown(event: PointerEvent) {
      if (headerRef.current?.contains(event.target as Node)) return;
      setMobileOpen(false);
      if (accountRef.current) accountRef.current.open = false;
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key !== "Escape") return;

      if (mobileOpen) {
        event.preventDefault();
        setMobileOpen(false);
        menuToggleRef.current?.focus();
        return;
      }

      if (accountRef.current?.open) {
        event.preventDefault();
        accountRef.current.open = false;
        accountTriggerRef.current?.focus();
      }
    }

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [mobileOpen]);

  useEffect(() => {
    const desktop = window.matchMedia("(min-width: 981px)");
    const handleDesktop = (event: MediaQueryListEvent) => {
      if (event.matches) setMobileOpen(false);
    };

    desktop.addEventListener("change", handleDesktop);
    return () => desktop.removeEventListener("change", handleDesktop);
  }, []);

  useEffect(() => {
    if (previousPathname.current === pathname) return;
    previousPathname.current = pathname;
    setMobileOpen(false);
    if (accountRef.current) accountRef.current.open = false;
  }, [pathname]);

  function closeAccountMenu() {
    if (accountRef.current) accountRef.current.open = false;
  }

  function closeMobileMenu() {
    setMobileOpen(false);
  }

  async function handleSignOut() {
    setSigningOut(true);
    closeAccountMenu();
    closeMobileMenu();
    try {
      await signOut();
    } finally {
      setSigningOut(false);
    }
  }

  return (
    <header ref={headerRef} className={styles.header}>
      <Shell className={styles.bar}>
        <Link href="/" className={styles.brand} aria-label="Illarin home">
          <BrandMark size={26} />
          <span className={styles.wordmark}>Illarin</span>
        </Link>

        <nav className={styles.nav} aria-label="Primary">
          {NAV.map((item) => (
            <Link
              key={item.label}
              href={item.href}
              className={styles.navLink}
              aria-current={
                isCurrentPage(pathname, item.href) ? "page" : undefined
              }
            >
              {item.label}
            </Link>
          ))}
        </nav>

        <div className={styles.actions}>
          <Link
            href={publishHref}
            className={styles.publish}
            aria-current={pathname === "/upload" ? "page" : undefined}
          >
            <Upload size={15} strokeWidth={1.6} aria-hidden="true" />
            {publishLabel}
          </Link>

          <details ref={accountRef} className={styles.account}>
            <summary ref={accountTriggerRef} className={styles.accountTrigger}>
              <CircleUserRound
                size={18}
                strokeWidth={1.55}
                aria-hidden="true"
              />
              <span>
                {account === undefined
                  ? "Account"
                  : account
                    ? `@${account.handle}`
                    : "Account"}
              </span>
            </summary>
            <div className={styles.accountMenu}>
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
                  <Link
                    href={`/@${account.handle}`}
                    aria-current={
                      isCurrentPage(pathname, `/@${account.handle}`)
                        ? "page"
                        : undefined
                    }
                    onClick={closeAccountMenu}
                  >
                    View profile
                  </Link>
                  <Link
                    href="/settings"
                    aria-current={
                      isCurrentPage(pathname, "/settings") ? "page" : undefined
                    }
                    onClick={closeAccountMenu}
                  >
                    Account settings
                  </Link>
                  {!account.emailVerified ? (
                    <Link
                      href={`/verify-email?returnTo=${UPLOAD_RETURN}`}
                      aria-current={
                        isCurrentPage(pathname, "/verify-email")
                          ? "page"
                          : undefined
                      }
                      onClick={closeAccountMenu}
                    >
                      Verify email
                    </Link>
                  ) : null}
                </>
              ) : (
                <>
                  <Link
                    href="/sign-in"
                    aria-current={
                      isCurrentPage(pathname, "/sign-in") ? "page" : undefined
                    }
                    onClick={closeAccountMenu}
                  >
                    Sign in
                  </Link>
                  <Link
                    href="/sign-up"
                    aria-current={
                      isCurrentPage(pathname, "/sign-up") ? "page" : undefined
                    }
                    onClick={closeAccountMenu}
                  >
                    Create account
                  </Link>
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
        </div>

        <div className={styles.spacer} />

        <button
          type="button"
          ref={menuToggleRef}
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
        <nav
          ref={mobileNavRef}
          id="mobile-navigation"
          className={styles.mobileNav}
          aria-label="Primary and account"
        >
          <Shell className={styles.mobileNavInner}>
            {NAV.map((item) => (
              <Link
                key={item.label}
                href={item.href}
                aria-current={
                  isCurrentPage(pathname, item.href) ? "page" : undefined
                }
                onClick={closeMobileMenu}
              >
                {item.label}
              </Link>
            ))}
            <Link
              href={publishHref}
              className={styles.mobilePublish}
              aria-current={pathname === "/upload" ? "page" : undefined}
              onClick={closeMobileMenu}
            >
              {publishLabel}
            </Link>
            {account ? (
              <>
                <div className={styles.mobileAccountIdentity}>
                  <CircleUserRound
                    size={19}
                    strokeWidth={1.55}
                    aria-hidden="true"
                  />
                  <span>
                    <strong>@{account.handle}</strong>
                    <small>
                      {account.emailVerified
                        ? "Verified account"
                        : "Email verification needed"}
                    </small>
                  </span>
                </div>
                <Link
                  href={`/@${account.handle}`}
                  aria-current={
                    isCurrentPage(pathname, `/@${account.handle}`)
                      ? "page"
                      : undefined
                  }
                  onClick={closeMobileMenu}
                >
                  View profile
                </Link>
                <Link
                  href="/settings"
                  aria-current={
                    isCurrentPage(pathname, "/settings") ? "page" : undefined
                  }
                  onClick={closeMobileMenu}
                >
                  Account settings
                </Link>
                {!account.emailVerified ? (
                  <Link
                    href={`/verify-email?returnTo=${UPLOAD_RETURN}`}
                    aria-current={
                      isCurrentPage(pathname, "/verify-email")
                        ? "page"
                        : undefined
                    }
                    onClick={closeMobileMenu}
                  >
                    Verify email
                  </Link>
                ) : null}
              </>
            ) : (
              <>
                <Link
                  href="/sign-in"
                  aria-current={
                    isCurrentPage(pathname, "/sign-in") ? "page" : undefined
                  }
                  onClick={closeMobileMenu}
                >
                  Sign in
                </Link>
                <Link
                  href="/sign-up"
                  aria-current={
                    isCurrentPage(pathname, "/sign-up") ? "page" : undefined
                  }
                  onClick={closeMobileMenu}
                >
                  Create account
                </Link>
              </>
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
