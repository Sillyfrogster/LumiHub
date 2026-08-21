"use client";

import { Check, Mail } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { type FormEvent, useEffect, useRef, useState } from "react";
import { useAuth } from "@/lib/auth";
import { safeInternalReturnPath } from "@/lib/internal-return";
import styles from "./VerificationPanel.module.css";

type VerificationState = "checking" | "waiting" | "verified" | "error";

export function VerificationPanel() {
  const search = useSearchParams();
  const router = useRouter();
  const { account, refresh } = useAuth();
  const token = search.get("token");
  const returnTo = safeInternalReturnPath(search.get("returnTo")) ?? "/browse";
  const returnLabel = returnTo.startsWith("/link")
    ? "Return to linking"
    : "Browse Illarin";
  const started = useRef(false);
  const [state, setState] = useState<VerificationState>(
    token ? "checking" : "waiting",
  );
  const [message, setMessage] = useState("");
  const [changing, setChanging] = useState(false);

  useEffect(() => {
    if (!token || started.current) return;
    started.current = true;

    async function verify() {
      try {
        const response = await fetch("/api/v1/auth/verify-email", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        });
        const answer = (await response.json()) as { error?: string };
        if (!response.ok) {
          setMessage(
            answer.error ?? "This verification link could not be used.",
          );
          setState("error");
          return;
        }
        await refresh();
        setState("verified");
        router.replace(
          returnTo === "/browse"
            ? "/verify-email?verified=1"
            : `/verify-email?verified=1&returnTo=${encodeURIComponent(returnTo)}`,
        );
      } catch {
        setMessage(
          "We could not reach Illarin. Check your connection and try the link again.",
        );
        setState("error");
      }
    }

    void verify();
  }, [refresh, returnTo, router, token]);

  async function changeEmail(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setChanging(true);
    setMessage("");
    const form = new FormData(event.currentTarget);

    try {
      const response = await fetch("/api/v1/account/email", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ email: String(form.get("email") ?? "") }),
      });
      const answer = (await response.json()) as { error?: string };
      if (!response.ok) {
        setMessage(answer.error ?? "The email address could not be changed.");
        return;
      }
      await refresh();
      event.currentTarget.reset();
      setMessage("A fresh verification link is on its way.");
    } catch {
      setMessage(
        "We could not reach Illarin. Check your connection and try again.",
      );
    } finally {
      setChanging(false);
    }
  }

  if (state === "checking") {
    return (
      <section className={styles.panel} aria-live="polite">
        <Mail size={25} strokeWidth={1.3} />
        <h2>Verifying your address…</h2>
        <p>The page will settle in just a moment.</p>
      </section>
    );
  }

  if (state === "verified" || (!token && account?.emailVerified)) {
    return (
      <section className={styles.panel}>
        <span className={styles.successMark}>
          <Check size={24} strokeWidth={1.8} />
        </span>
        <h2>Your address is verified</h2>
        <p>Your handle is yours, and you can now publish work under it.</p>
        <Link className={styles.primaryLink} href={returnTo}>
          {returnLabel}
        </Link>
      </section>
    );
  }

  if (state === "error") {
    return (
      <section className={styles.panel}>
        <Mail size={25} strokeWidth={1.3} />
        <h2>This link did not open</h2>
        <p className={styles.error} role="alert">
          {message}
        </p>
        <Link className={styles.primaryLink} href="/sign-in">
          Return to sign in
        </Link>
      </section>
    );
  }

  return (
    <section className={styles.panel}>
      <Mail size={25} strokeWidth={1.3} />
      <h2>Check your email</h2>
      <p>
        We sent a verification link
        {account?.email ? ` to ${account.email}` : " to your address"}. You can
        keep browsing while you wait.
      </p>

      {account && !account.emailVerified ? (
        <form className={styles.changeForm} onSubmit={changeEmail}>
          <label htmlFor="corrected-email">Mistyped the address?</label>
          <div className={styles.changeRow}>
            <input
              id="corrected-email"
              name="email"
              type="email"
              autoComplete="email"
              placeholder="Correct email address"
              required
            />
            <button type="submit" disabled={changing}>
              {changing ? "Sending…" : "Send a new link"}
            </button>
          </div>
        </form>
      ) : null}

      {message ? <output className={styles.message}>{message}</output> : null}
      <Link className={styles.secondaryLink} href="/browse">
        Browse while you wait
      </Link>
    </section>
  );
}
