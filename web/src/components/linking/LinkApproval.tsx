"use client";

import { Check, Plug, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import type { components } from "@/lib/api/schema";
import { useAuth } from "@/lib/auth";
import { describeScope } from "@/lib/scopes";
import styles from "./LinkApproval.module.css";

type PendingLink = components["schemas"]["PendingLink"];

type Stage =
  | { kind: "entry" }
  | { kind: "looking" }
  | { kind: "confirm"; code: string; link: PendingLink }
  | { kind: "approving"; code: string; link: PendingLink }
  | { kind: "approved"; link: PendingLink };

const UNREACHABLE =
  "We could not reach LumiHub. Check your connection and try again.";

export function LinkApproval() {
  const search = useSearchParams();
  const { account } = useAuth();
  const suppliedCode = search.get("code") ?? "";
  const [stage, setStage] = useState<Stage>({ kind: "entry" });
  const [notice, setNotice] = useState("");
  const looked = useRef("");

  const look = useCallback(async (code: string) => {
    setNotice("");
    setStage({ kind: "looking" });
    try {
      const response = await fetch(
        `/api/v1/link/requests/${encodeURIComponent(code)}`,
        { cache: "no-store", credentials: "same-origin" },
      );
      const answer = (await response.json()) as PendingLink & {
        error?: string;
      };
      if (!response.ok) {
        setNotice(answer.error ?? "That code does not match a link request.");
        setStage({ kind: "entry" });
        return;
      }
      setStage({ kind: "confirm", code, link: answer });
    } catch {
      setNotice(UNREACHABLE);
      setStage({ kind: "entry" });
    }
  }, []);

  useEffect(() => {
    if (!suppliedCode || !account?.emailVerified) return;
    if (looked.current === suppliedCode) return;
    looked.current = suppliedCode;
    void look(suppliedCode);
  }, [suppliedCode, account, look]);

  if (account === undefined) {
    return (
      <div className={styles.panel}>
        <p className={styles.waiting} aria-live="polite">
          Checking who is signed in…
        </p>
      </div>
    );
  }

  if (!account) {
    return (
      <div className={styles.panel}>
        <Gate
          title="Sign in to approve"
          body="An application can only be linked by the creator whose account it will reach."
          code={suppliedCode}
        />
        <Link className={styles.primary} href="/sign-in">
          Sign in
        </Link>
      </div>
    );
  }

  if (!account.emailVerified) {
    return (
      <div className={styles.panel}>
        <Gate
          title="Verify your email first"
          body="A verified address is what lets you cut a link later, so it is needed before one is made."
          code={suppliedCode}
        />
        <Link className={styles.primary} href="/verify-email">
          Verify email
        </Link>
      </div>
    );
  }

  if (stage.kind === "approved") {
    return (
      <div className={styles.panel}>
        <span className={styles.done}>
          <Check size={26} strokeWidth={1.6} aria-hidden="true" />
        </span>
        <h2 className={styles.title}>{stage.link.name} is linked</h2>
        <p className={styles.body}>
          Go back to it. It picks this up within a few seconds. You can cut the
          link at any time from your settings.
        </p>
        <Link className={styles.primary} href="/settings">
          See linked instances
        </Link>
      </div>
    );
  }

  if (stage.kind === "confirm" || stage.kind === "approving") {
    return (
      <Confirmation
        stage={stage}
        notice={notice}
        onApprove={async () => {
          setNotice("");
          setStage({ kind: "approving", code: stage.code, link: stage.link });
          try {
            const response = await fetch(
              `/api/v1/link/requests/${encodeURIComponent(stage.code)}/approve`,
              { method: "POST", credentials: "same-origin" },
            );
            const answer = (await response.json()) as PendingLink & {
              error?: string;
            };
            if (!response.ok) {
              setNotice(
                answer.error ?? "This link request could not be approved.",
              );
              setStage({ kind: "entry" });
              return;
            }
            setStage({ kind: "approved", link: answer });
          } catch {
            setNotice(UNREACHABLE);
            setStage({ kind: "confirm", code: stage.code, link: stage.link });
          }
        }}
        onCancel={() => {
          looked.current = "";
          setStage({ kind: "entry" });
        }}
      />
    );
  }

  return (
    <form
      className={styles.panel}
      onSubmit={(event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const typed = String(
          new FormData(event.currentTarget).get("code") ?? "",
        );
        looked.current = typed;
        void look(typed);
      }}
      noValidate
    >
      <span className={styles.mark}>
        <Plug size={24} strokeWidth={1.4} aria-hidden="true" />
      </span>
      <h2 className={styles.title}>Enter the code</h2>
      <p className={styles.body}>
        Your application is showing eight characters. Type them here and you
        will see what it is asking for before anything is granted.
      </p>
      <label className="sr-only" htmlFor="link-code">
        Link code
      </label>
      <input
        id="link-code"
        name="code"
        className={styles.code}
        defaultValue={suppliedCode}
        placeholder="XXXX-XXXX"
        autoComplete="off"
        autoCapitalize="characters"
        spellCheck={false}
        maxLength={12}
        required
      />
      {notice ? (
        <output className={styles.notice} aria-live="polite">
          {notice}
        </output>
      ) : null}
      <button
        className={styles.primary}
        type="submit"
        disabled={stage.kind === "looking"}
      >
        {stage.kind === "looking" ? "Looking…" : "Continue"}
      </button>
    </form>
  );
}

function Gate({
  title,
  body,
  code,
}: {
  title: string;
  body: string;
  code: string;
}) {
  return (
    <>
      <span className={styles.mark}>
        <ShieldCheck size={24} strokeWidth={1.4} aria-hidden="true" />
      </span>
      <h2 className={styles.title}>{title}</h2>
      <p className={styles.body}>{body}</p>
      {code ? (
        <p className={styles.carried}>
          Come back here afterwards with the code your application is showing.
          It was <strong>{code}</strong>.
        </p>
      ) : null}
    </>
  );
}

function Confirmation({
  stage,
  notice,
  onApprove,
  onCancel,
}: {
  stage: { kind: "confirm" | "approving"; code: string; link: PendingLink };
  notice: string;
  onApprove: () => void;
  onCancel: () => void;
}) {
  return (
    <div className={styles.panel}>
      <p className={styles.asking}>This application is asking to link</p>
      <h2 className={styles.name}>{stage.link.name}</h2>
      <p className={styles.codeShown}>{stage.code}</p>

      <ul className={styles.scopes}>
        {stage.link.scopes.map((scope) => {
          const copy = describeScope(scope);
          return (
            <li key={scope}>
              <Check size={16} strokeWidth={2} aria-hidden="true" />
              <div>
                <strong>{copy.title}</strong>
                <span>{copy.detail}</span>
              </div>
            </li>
          );
        })}
      </ul>

      <p className={styles.caution}>
        The name above was written by the application, not by LumiHub. Approve
        this only if you started it yourself.
      </p>

      {notice ? (
        <output className={styles.notice} aria-live="polite">
          {notice}
        </output>
      ) : null}

      <div className={styles.decision}>
        <button
          className={styles.primary}
          type="button"
          onClick={onApprove}
          disabled={stage.kind === "approving"}
        >
          {stage.kind === "approving" ? "Approving…" : "Approve"}
        </button>
        <button className={styles.secondary} type="button" onClick={onCancel}>
          Not mine
        </button>
      </div>
    </div>
  );
}
