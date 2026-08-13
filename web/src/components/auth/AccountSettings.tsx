"use client";

import {
  Check,
  KeyRound,
  Mail,
  MessageCircle,
  ShieldCheck,
} from "lucide-react";
import Link from "next/link";
import { type FormEvent, useState } from "react";
import type { SignedInAccount } from "@/lib/auth";
import { useAuth } from "@/lib/auth";
import styles from "./AccountSettings.module.css";

type ErrorAnswer = { error?: string };

export function AccountSettings({ discordNotice }: { discordNotice?: string }) {
  const { account, setAccount } = useAuth();
  const [message, setMessage] = useState(discordNotice ?? "");
  const [passwordPending, setPasswordPending] = useState(false);
  const [detachPending, setDetachPending] = useState(false);

  if (account === undefined) {
    return (
      <div className={styles.loading} aria-live="polite">
        <span />
        <span />
        <span />
        <p>Reading your sign-in methods…</p>
      </div>
    );
  }

  if (!account) {
    return (
      <section className={styles.signedOut}>
        <ShieldCheck size={27} strokeWidth={1.35} aria-hidden="true" />
        <h2>Sign in to open account settings</h2>
        <p>Your sign-in methods belong behind the same private session.</p>
        <Link href="/sign-in">Sign in</Link>
      </section>
    );
  }

  async function setPassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const hadPassword = account?.hasPassword ?? false;
    setPasswordPending(true);
    setMessage("");
    const form = new FormData(event.currentTarget);

    try {
      const response = await fetch("/api/v1/account/password", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({
          password: String(form.get("password") ?? ""),
        }),
      });
      const answer = (await response.json()) as SignedInAccount & ErrorAnswer;
      if (!response.ok) {
        setMessage(answer.error ?? "The password could not be saved.");
        return;
      }
      setAccount(answer);
      event.currentTarget.reset();
      setMessage(
        hadPassword
          ? "Your password has been changed."
          : "Your email can now be used with this password.",
      );
    } catch {
      setMessage(
        "We could not reach LumiHub. Check your connection and try again.",
      );
    } finally {
      setPasswordPending(false);
    }
  }

  async function detachDiscord() {
    setDetachPending(true);
    setMessage("");
    try {
      const response = await fetch("/api/v1/account/discord", {
        method: "DELETE",
        credentials: "same-origin",
      });
      const answer = (await response.json()) as SignedInAccount & ErrorAnswer;
      if (!response.ok) {
        setMessage(answer.error ?? "Discord could not be detached.");
        return;
      }
      setAccount(answer);
      setMessage("Discord is detached and free to be used on another account.");
    } catch {
      setMessage(
        "We could not reach LumiHub. Check your connection and try again.",
      );
    } finally {
      setDetachPending(false);
    }
  }

  const canDetach = account.emailVerified && account.hasPassword;

  return (
    <section className={styles.settings}>
      <div className={styles.summary}>
        <div>
          <p>Signed in as</p>
          <h2>@{account.handle}</h2>
        </div>
        <span className={styles.safety}>
          <ShieldCheck size={17} strokeWidth={1.55} aria-hidden="true" />
          {account.emailVerified ? "Verified account" : "Browse-only account"}
        </span>
      </div>

      {message ? (
        <output className={styles.message} aria-live="polite">
          {message}
        </output>
      ) : null}

      <div className={styles.method}>
        <span className={styles.methodIcon}>
          <Mail size={21} strokeWidth={1.4} aria-hidden="true" />
        </span>
        <div className={styles.methodCopy}>
          <h3>Email address</h3>
          <p>{account.email ?? "No verified address yet"}</p>
          <span>
            {account.emailVerified
              ? "Verified and available for recovery."
              : "Verify an address before publishing or detaching Discord."}
          </span>
        </div>
        {!account.emailVerified ? (
          <Link className={styles.secondaryAction} href="/verify-email">
            Add or verify email
          </Link>
        ) : (
          <span className={styles.complete}>
            <Check size={15} strokeWidth={1.8} aria-hidden="true" />
            Verified
          </span>
        )}
      </div>

      <div className={styles.method}>
        <span className={styles.methodIcon}>
          <MessageCircle size={21} strokeWidth={1.4} aria-hidden="true" />
        </span>
        <div className={styles.methodCopy}>
          <h3>Discord</h3>
          <p>{account.discordLinked ? "Attached" : "Not attached"}</p>
          <span>
            {account.discordLinked
              ? "Use Discord to return without entering a password."
              : "Attach one Discord identity without combining accounts."}
          </span>
        </div>
        {account.discordLinked ? (
          <button
            className={styles.secondaryAction}
            type="button"
            onClick={detachDiscord}
            disabled={!canDetach || detachPending}
            aria-describedby={!canDetach ? "detach-requirement" : undefined}
          >
            {detachPending ? "Detaching…" : "Detach Discord"}
          </button>
        ) : (
          <a
            className={styles.primaryAction}
            href="/api/v1/auth/discord?intent=attach"
            aria-disabled={!account.emailVerified}
            onClick={(event) => {
              if (!account.emailVerified) event.preventDefault();
            }}
          >
            Attach Discord
          </a>
        )}
      </div>

      {account.discordLinked && !canDetach ? (
        <p id="detach-requirement" className={styles.requirement}>
          Keep at least one verified way in. Verify an email and add a password
          before detaching Discord.
        </p>
      ) : null}

      <form className={styles.password} onSubmit={setPassword} noValidate>
        <span className={styles.methodIcon}>
          <KeyRound size={21} strokeWidth={1.4} aria-hidden="true" />
        </span>
        <div className={styles.methodCopy}>
          <h3>{account.hasPassword ? "Change password" : "Add a password"}</h3>
          <p>
            {account.hasPassword
              ? "Replace the password used with your verified email."
              : "Create an independent way back if Discord is unavailable."}
          </p>
        </div>
        <div className={styles.passwordControl}>
          <label className="sr-only" htmlFor="settings-password">
            {account.hasPassword ? "New password" : "Password"}
          </label>
          <input
            id="settings-password"
            name="password"
            type="password"
            autoComplete="new-password"
            placeholder={account.hasPassword ? "New password" : "Password"}
            required
          />
          <button type="submit" disabled={passwordPending}>
            {passwordPending
              ? "Saving…"
              : account.hasPassword
                ? "Change password"
                : "Add password"}
          </button>
        </div>
      </form>
    </section>
  );
}
