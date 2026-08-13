"use client";

import { MessageCircle } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import type { SignedInAccount } from "@/lib/auth";
import { useAuth } from "@/lib/auth";
import styles from "./AccountForm.module.css";

type ErrorAnswer = { error?: string; field?: string };

export function AccountForm({
  mode,
  discordError,
}: {
  mode: "sign-in" | "sign-up";
  discordError?: string;
}) {
  const router = useRouter();
  const { setAccount } = useAuth();
  const [error, setError] = useState<ErrorAnswer | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);

    const form = new FormData(event.currentTarget);
    const body: Record<string, string> = {
      email: String(form.get("email") ?? ""),
      password: String(form.get("password") ?? ""),
    };
    if (mode === "sign-up") body.handle = String(form.get("handle") ?? "");

    try {
      const response = await fetch(
        mode === "sign-up" ? "/api/v1/auth/sign-up" : "/api/v1/auth/sign-in",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "same-origin",
          body: JSON.stringify(body),
        },
      );
      const answer = (await response.json()) as SignedInAccount & ErrorAnswer;
      if (!response.ok) {
        setError(answer);
        return;
      }
      setAccount(answer);
      router.push(mode === "sign-up" ? "/verify-email" : "/browse");
    } catch {
      setError({
        error:
          "We could not reach LumiHub. Check your connection and try again.",
      });
    } finally {
      setPending(false);
    }
  }

  const signUp = mode === "sign-up";

  return (
    <form className={styles.form} onSubmit={submit} noValidate>
      <div className={styles.headingGroup}>
        <h2>{signUp ? "Begin your profile" : "Welcome back"}</h2>
        <p>
          {signUp
            ? "One address, one private password and the name readers will know."
            : "Return to the work and worlds you have gathered here."}
        </p>
      </div>

      <Link className={styles.discord} href="/api/v1/auth/discord">
        <MessageCircle size={18} strokeWidth={1.7} aria-hidden="true" />
        Continue with Discord
      </Link>

      {discordError ? (
        <p className={styles.error} role="alert">
          {discordError}
        </p>
      ) : null}

      <div className={styles.divider} aria-hidden="true">
        <span />
        <p>or use email</p>
        <span />
      </div>

      {signUp ? (
        <div className={styles.field}>
          <label htmlFor="account-handle">Handle</label>
          <div
            className={styles.handleField}
            data-error={error?.field === "handle" ? "true" : undefined}
          >
            <span aria-hidden="true">@</span>
            <input
              id="account-handle"
              name="handle"
              type="text"
              minLength={3}
              maxLength={32}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              aria-describedby="handle-hint"
              aria-invalid={error?.field === "handle" || undefined}
              required
            />
          </div>
          <p id="handle-hint" className={styles.hint}>
            3–32 lowercase letters, numbers, dots or underscores.
          </p>
        </div>
      ) : null}

      <div className={styles.field}>
        <label htmlFor="account-email">Email address</label>
        <input
          id="account-email"
          name="email"
          type="email"
          autoComplete="email"
          autoCapitalize="none"
          spellCheck={false}
          aria-invalid={error?.field === "email" || undefined}
          required
        />
      </div>

      <div className={styles.field}>
        <div className={styles.fieldLabel}>
          <label htmlFor="account-password">Password</label>
          {!signUp ? (
            <Link href="/forgot-password">Forgot password?</Link>
          ) : null}
        </div>
        <input
          id="account-password"
          name="password"
          type="password"
          autoComplete={signUp ? "new-password" : "current-password"}
          aria-invalid={error?.field === "password" || undefined}
          required
        />
      </div>

      {error?.error ? (
        <p className={styles.error} role="alert">
          {error.error}
        </p>
      ) : null}

      <button className={styles.submit} type="submit" disabled={pending}>
        {pending
          ? signUp
            ? "Creating your account…"
            : "Signing you in…"
          : signUp
            ? "Create account"
            : "Sign in"}
      </button>

      <p className={styles.alternative}>
        {signUp ? "Already have an account?" : "New to LumiHub?"}{" "}
        <Link href={signUp ? "/sign-in" : "/sign-up"}>
          {signUp ? "Sign in" : "Create an account"}
        </Link>
      </p>
    </form>
  );
}
