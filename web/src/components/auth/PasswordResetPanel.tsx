"use client";

import { Check, KeyRound, Mail } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { type FormEvent, useState } from "react";
import styles from "./PasswordResetPanel.module.css";

type ErrorAnswer = { error?: string };

export function PasswordResetRequestPanel() {
  const [pending, setPending] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");
    const form = new FormData(event.currentTarget);

    try {
      const response = await fetch("/api/v1/auth/password-reset", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: String(form.get("email") ?? "") }),
      });
      if (!response.ok) {
        const answer = (await response.json()) as ErrorAnswer;
        setError(answer.error ?? "The reset request could not be sent.");
        return;
      }
      setSent(true);
    } catch {
      setError(
        "We could not reach LumiHub. Check your connection and try again.",
      );
    } finally {
      setPending(false);
    }
  }

  if (sent) {
    return (
      <section className={styles.confirmation} aria-live="polite">
        <Mail size={25} strokeWidth={1.4} aria-hidden="true" />
        <h2>Check your email</h2>
        <p>
          If that verified address belongs to an account, a one-use password
          link is on its way.
        </p>
        <Link href="/sign-in">Return to sign in</Link>
      </section>
    );
  }

  return (
    <form className={styles.form} onSubmit={submit} noValidate>
      <div className={styles.headingGroup}>
        <KeyRound size={24} strokeWidth={1.35} aria-hidden="true" />
        <h2>Find your account</h2>
        <p>
          This also works if Discord has been your only way into LumiHub until
          now.
        </p>
      </div>
      <div className={styles.field}>
        <label htmlFor="reset-email">Verified email address</label>
        <input
          id="reset-email"
          name="email"
          type="email"
          autoComplete="email"
          autoCapitalize="none"
          spellCheck={false}
          required
        />
      </div>
      {error ? (
        <p className={styles.error} role="alert">
          {error}
        </p>
      ) : null}
      <button type="submit" disabled={pending}>
        {pending ? "Sending a reset link…" : "Send reset link"}
      </button>
      <Link className={styles.secondary} href="/sign-in">
        Return to sign in
      </Link>
    </form>
  );
}

export function PasswordResetCompletionPanel() {
  const search = useSearchParams();
  const token = search.get("token");
  const [pending, setPending] = useState(false);
  const [complete, setComplete] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) return;
    setPending(true);
    setError("");
    const form = new FormData(event.currentTarget);

    try {
      const response = await fetch("/api/v1/auth/password-reset/complete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          token,
          password: String(form.get("password") ?? ""),
        }),
      });
      if (!response.ok) {
        const answer = (await response.json()) as ErrorAnswer;
        setError(answer.error ?? "This password reset link could not be used.");
        return;
      }
      setComplete(true);
    } catch {
      setError(
        "We could not reach LumiHub. Check your connection and try again.",
      );
    } finally {
      setPending(false);
    }
  }

  if (complete) {
    return (
      <section className={styles.confirmation} aria-live="polite">
        <span className={styles.successMark}>
          <Check size={23} strokeWidth={1.8} aria-hidden="true" />
        </span>
        <h2>Your password is ready</h2>
        <p>
          You can now return with your verified email, even without Discord.
        </p>
        <Link href="/sign-in">Sign in with email</Link>
      </section>
    );
  }

  if (!token) {
    return (
      <section className={styles.confirmation}>
        <KeyRound size={25} strokeWidth={1.35} aria-hidden="true" />
        <h2>This link is incomplete</h2>
        <p>Request a fresh password link and open it from your email.</p>
        <Link href="/forgot-password">Request another link</Link>
      </section>
    );
  }

  return (
    <form className={styles.form} onSubmit={submit} noValidate>
      <div className={styles.headingGroup}>
        <KeyRound size={24} strokeWidth={1.35} aria-hidden="true" />
        <h2>Choose a new password</h2>
        <p>The link can be used once. Your new password may be any length.</p>
      </div>
      <div className={styles.field}>
        <label htmlFor="reset-password">New password</label>
        <input
          id="reset-password"
          name="password"
          type="password"
          autoComplete="new-password"
          required
        />
      </div>
      {error ? (
        <p className={styles.error} role="alert">
          {error}
        </p>
      ) : null}
      <button type="submit" disabled={pending}>
        {pending ? "Setting your password…" : "Set password"}
      </button>
    </form>
  );
}
