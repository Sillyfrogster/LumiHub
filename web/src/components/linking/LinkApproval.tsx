"use client";

import { Check, CircleX, Plug, ShieldAlert, ShieldCheck } from "lucide-react";
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
type PendingDeviceLink = components["schemas"]["PendingDeviceLink"];
type LinkRedirect = components["schemas"]["LinkRedirect"];

type ReviewRequest =
  | { kind: "authorization"; requestCode: string }
  | { kind: "device"; userCode: string };

type ReviewSource =
  | { kind: "authorization"; requestCode: string }
  | { kind: "device"; userCode: string; approvalToken: string };

type Review = { source: ReviewSource; link: PendingLink };
type Decision = "approve" | "deny";

type Stage =
  | { kind: "entry" }
  | { kind: "loading" }
  | { kind: "request-error" }
  | { kind: "confirm"; review: Review }
  | { kind: "deciding"; review: Review; decision: Decision }
  | { kind: "redirecting"; review: Review; decision: Decision }
  | { kind: "approved"; link: PendingLink }
  | { kind: "denied"; link: PendingLink };

const UNREACHABLE =
  "We could not reach Illarin. Check your connection and try again.";
const BROWSER_MUTATION_HEADER = { "X-Illarin-Request": "1" } as const;

export function LinkApproval() {
  const search = useSearchParams();
  const { account } = useAuth();
  const requestCode = search.get("request")?.trim() ?? "";
  const returnTo = requestCode
    ? `/link?request=${encodeURIComponent(requestCode)}`
    : "/link";
  const [manualForRequest, setManualForRequest] = useState("");
  const [stage, setStage] = useState<Stage>({ kind: "entry" });
  const [typed, setTyped] = useState("");
  const [notice, setNotice] = useState("");
  const looked = useRef("");
  const activePanel = useRef<HTMLElement | null>(null);
  const previousView = useRef("");
  const reviewingRequest =
    requestCode !== "" && manualForRequest !== requestCode;
  const stageView =
    stage.kind === "deciding" || stage.kind === "redirecting"
      ? "confirm"
      : stage.kind;
  const viewKey =
    account === undefined
      ? "checking-account"
      : !account
        ? "signed-out"
        : !account.emailVerified
          ? "unverified"
          : stageView;
  const capturePanel = useCallback((node: HTMLElement | null) => {
    activePanel.current = node;
  }, []);

  useEffect(() => {
    if (previousView.current && previousView.current !== viewKey) {
      activePanel.current?.focus();
    }
    previousView.current = viewKey;
  }, [viewKey]);

  const loadReview = useCallback(async (request: ReviewRequest) => {
    setNotice("");
    setStage({ kind: "loading" });

    const isAuthorization = request.kind === "authorization";
    const endpoint = isAuthorization
      ? `/api/v1/link/authorizations/${encodeURIComponent(request.requestCode)}`
      : `/api/v1/link/requests/${encodeURIComponent(request.userCode)}`;

    try {
      const response = await fetch(endpoint, {
        cache: "no-store",
        credentials: "same-origin",
      });
      const answer: unknown = await readJSON(response);
      if (!response.ok) {
        setNotice(
          errorMessage(
            answer,
            isAuthorization
              ? "That browser link request is no longer available."
              : "That code does not match a pending link request.",
          ),
        );
        setStage({ kind: isAuthorization ? "request-error" : "entry" });
        return;
      }

      if (isAuthorization) {
        if (!isPendingLink(answer)) {
          setNotice("Illarin returned an incomplete link request. Try again.");
          setStage({ kind: "request-error" });
          return;
        }
        setStage({
          kind: "confirm",
          review: {
            source: {
              kind: "authorization",
              requestCode: request.requestCode,
            },
            link: answer,
          },
        });
        return;
      }

      if (!isPendingDeviceLink(answer)) {
        setNotice("Illarin returned an incomplete link request. Try again.");
        setStage({ kind: "entry" });
        return;
      }
      setStage({
        kind: "confirm",
        review: {
          source: {
            kind: "device",
            userCode: request.userCode,
            approvalToken: answer.approvalToken,
          },
          link: answer,
        },
      });
    } catch {
      setNotice(UNREACHABLE);
      setStage({ kind: isAuthorization ? "request-error" : "entry" });
    }
  }, []);

  useEffect(() => {
    if (!reviewingRequest || !account?.emailVerified) return;
    if (looked.current === requestCode) return;
    looked.current = requestCode;
    void loadReview({ kind: "authorization", requestCode });
  }, [requestCode, reviewingRequest, account, loadReview]);

  if (account === undefined) {
    return (
      <div
        ref={capturePanel}
        className={styles.panel}
        tabIndex={-1}
        aria-live="polite"
      >
        <p className={styles.waiting}>Checking who is signed in…</p>
      </div>
    );
  }

  if (!account) {
    return (
      <div
        ref={capturePanel}
        className={styles.panel}
        tabIndex={-1}
        aria-live="polite"
      >
        <Gate
          title="Sign in to review this link"
          body="An application can only be linked by the creator whose account it will reach."
          requestPending={reviewingRequest}
        />
        <Link
          className={styles.primary}
          href={`/sign-in?returnTo=${encodeURIComponent(returnTo)}`}
        >
          Sign in
        </Link>
      </div>
    );
  }

  if (!account.emailVerified) {
    return (
      <div
        ref={capturePanel}
        className={styles.panel}
        tabIndex={-1}
        aria-live="polite"
      >
        <Gate
          title="Verify your email first"
          body="A verified address is needed before an application can be linked to your account."
          requestPending={reviewingRequest}
        />
        <Link
          className={styles.primary}
          href={`/verify-email?returnTo=${encodeURIComponent(returnTo)}`}
        >
          Verify email
        </Link>
      </div>
    );
  }

  if (stage.kind === "approved") {
    return (
      <div
        ref={capturePanel}
        className={styles.panel}
        tabIndex={-1}
        aria-live="polite"
      >
        <span className={styles.done}>
          <Check size={26} strokeWidth={1.6} aria-hidden="true" />
        </span>
        <h2 className={styles.title}>{stage.link.instanceName} is linked</h2>
        <p className={styles.body}>
          Go back to {stage.link.applicationName}. It can finish with a
          short-lived access token and a rotating refresh credential. This
          installation stays independent from every other link.
        </p>
        <Link className={styles.primary} href="/settings">
          See linked instances
        </Link>
      </div>
    );
  }

  if (stage.kind === "denied") {
    return (
      <div
        ref={capturePanel}
        className={styles.panel}
        tabIndex={-1}
        aria-live="polite"
      >
        <span className={styles.mark}>
          <CircleX size={24} strokeWidth={1.5} aria-hidden="true" />
        </span>
        <h2 className={styles.title}>Link declined</h2>
        <p className={styles.body}>
          {stage.link.applicationName} was not linked. The application will see
          that this request was denied.
        </p>
        <button
          className={styles.secondaryStandalone}
          type="button"
          onClick={() => {
            setNotice("");
            setTyped("");
            setStage({ kind: "entry" });
          }}
        >
          Enter another code
        </button>
      </div>
    );
  }

  if (
    stage.kind === "confirm" ||
    stage.kind === "deciding" ||
    stage.kind === "redirecting"
  ) {
    const review = stage.review;
    const decision = stage.kind === "confirm" ? null : stage.decision;
    return (
      <Confirmation
        review={review}
        decision={decision}
        notice={notice}
        panelRef={capturePanel}
        onDecision={(nextDecision) => {
          void decide(review, nextDecision, setStage, setNotice);
        }}
        onCancel={() => {
          setNotice("");
          setTyped("");
          setStage({ kind: "entry" });
        }}
      />
    );
  }

  if (stage.kind === "loading") {
    return (
      <div
        ref={capturePanel}
        className={styles.panel}
        tabIndex={-1}
        aria-live="polite"
        aria-busy="true"
      >
        <p className={styles.waiting}>Reading the link request…</p>
      </div>
    );
  }

  if (stage.kind === "request-error" && reviewingRequest) {
    return (
      <div
        ref={capturePanel}
        className={styles.panel}
        tabIndex={-1}
        aria-live="polite"
      >
        <span className={styles.mark}>
          <ShieldAlert size={24} strokeWidth={1.5} aria-hidden="true" />
        </span>
        <h2 className={styles.title}>This request could not be opened</h2>
        <p className={styles.body}>
          It may have expired or already been used. Reopen the link from your
          application, or enter a device code instead.
        </p>
        {notice ? <Notice>{notice}</Notice> : null}
        <div className={styles.decision}>
          <button
            className={styles.primary}
            type="button"
            onClick={() => {
              looked.current = requestCode;
              void loadReview({ kind: "authorization", requestCode });
            }}
          >
            Try again
          </button>
          <button
            className={styles.secondary}
            type="button"
            onClick={() => {
              setManualForRequest(requestCode);
              setNotice("");
              setStage({ kind: "entry" });
            }}
          >
            Enter a device code
          </button>
        </div>
      </div>
    );
  }

  return (
    <form
      ref={capturePanel}
      className={styles.panel}
      tabIndex={-1}
      aria-live="polite"
      onSubmit={(event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const userCode = typed.trim();
        if (!userCode) {
          setNotice("Enter the code shown by your application.");
          return;
        }
        void loadReview({ kind: "device", userCode });
      }}
      noValidate
    >
      <span className={styles.mark}>
        <Plug size={24} strokeWidth={1.4} aria-hidden="true" />
      </span>
      <h2 className={styles.title}>Enter the device code</h2>
      <p className={styles.body}>
        Type the eight characters shown by your application. You will review its
        identity and permissions before deciding.
      </p>
      <label className="sr-only" htmlFor="link-code">
        Device code
      </label>
      <input
        id="link-code"
        name="code"
        className={styles.code}
        value={typed}
        onChange={(event) => setTyped(event.target.value.toUpperCase())}
        placeholder="XXXX-XXXX"
        autoComplete="off"
        autoCapitalize="characters"
        spellCheck={false}
        maxLength={12}
        required
        aria-describedby={notice ? "link-entry-notice" : undefined}
      />
      {notice ? <Notice id="link-entry-notice">{notice}</Notice> : null}
      <button
        className={styles.primary}
        type="submit"
        disabled={typed.trim().length === 0}
      >
        Continue
      </button>
    </form>
  );
}

function Gate({
  title,
  body,
  requestPending,
}: {
  title: string;
  body: string;
  requestPending: boolean;
}) {
  return (
    <>
      <span className={styles.mark}>
        <ShieldCheck size={24} strokeWidth={1.4} aria-hidden="true" />
      </span>
      <h2 className={styles.title}>{title}</h2>
      <p className={styles.body}>{body}</p>
      {requestPending ? (
        <p className={styles.carried}>
          The browser request will remain available after this account step.
        </p>
      ) : null}
    </>
  );
}

function Confirmation({
  review,
  decision,
  notice,
  panelRef,
  onDecision,
  onCancel,
}: {
  review: Review;
  decision: Decision | null;
  notice: string;
  panelRef: (node: HTMLElement | null) => void;
  onDecision: (decision: Decision) => void;
  onCancel: () => void;
}) {
  const { link, source } = review;
  const busy = decision !== null;
  const isDevice = source.kind === "device";

  return (
    <div
      ref={panelRef}
      className={styles.panel}
      tabIndex={-1}
      aria-live="polite"
      aria-busy={busy}
    >
      <p className={styles.asking}>Unverified application</p>
      <h2 className={styles.name}>{link.applicationName}</h2>

      <dl className={styles.identity}>
        <div>
          <dt>Installation</dt>
          <dd>{link.instanceName}</dd>
        </div>
        {link.applicationVersion ? (
          <div>
            <dt>Version</dt>
            <dd>{link.applicationVersion}</dd>
          </div>
        ) : null}
        <div>
          <dt>Request expires</dt>
          <dd>
            <time dateTime={link.expiresAt}>
              {formatExpiry(link.expiresAt)}
            </time>
          </dd>
        </div>
      </dl>

      {isDevice ? (
        <div className={styles.codeMatch}>
          <ShieldAlert size={19} strokeWidth={1.7} aria-hidden="true" />
          <div>
            <strong>Match this code before continuing</strong>
            <span>
              Confirm <b>{source.userCode}</b> is exactly what the application
              shows. If it differs, decline.
            </span>
          </div>
        </div>
      ) : null}

      <section
        className={styles.permissions}
        aria-labelledby="link-permissions"
      >
        <h3 id="link-permissions">Permissions requested</h3>
        <ul className={styles.scopes}>
          {link.scopes.map((scope) => {
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
      </section>

      <CompatibilityDetails
        capabilities={link.capabilities}
        acceptedTargets={link.acceptedTargets}
        protocolVersion={link.protocolVersion}
      />

      <p className={styles.caution} id="unverified-link-details">
        These names and compatibility details were supplied by the application,
        not verified by Illarin. Approve only if you started this request.
      </p>

      {notice ? <Notice>{notice}</Notice> : null}

      <div
        className={styles.decision}
        aria-describedby="unverified-link-details"
      >
        <button
          className={styles.primary}
          type="button"
          onClick={() => onDecision("approve")}
          disabled={busy}
        >
          {decision === "approve" ? "Approving…" : "Approve"}
        </button>
        <button
          className={styles.secondary}
          type="button"
          onClick={() => onDecision("deny")}
          disabled={busy}
        >
          {decision === "deny" ? "Declining…" : "Decline"}
        </button>
      </div>

      {isDevice && !busy ? (
        <button className={styles.leave} type="button" onClick={onCancel}>
          Use a different code
        </button>
      ) : null}
    </div>
  );
}

function CompatibilityDetails({
  capabilities,
  acceptedTargets,
  protocolVersion,
}: {
  capabilities: string[];
  acceptedTargets: string[];
  protocolVersion: number;
}) {
  return (
    <section
      className={styles.compatibility}
      aria-labelledby="link-compatibility"
    >
      <div className={styles.compatibilityHeading}>
        <h3 id="link-compatibility">Interoperability</h3>
        <p>Self-reported technical details. They never grant permission.</p>
      </div>
      <dl className={styles.compatibilityList}>
        <TechnicalValues
          label="Accepted targets"
          values={acceptedTargets}
          empty="None declared"
        />
        <TechnicalValues
          label="Capabilities"
          values={capabilities}
          empty="None declared"
        />
        <div>
          <dt>Protocol</dt>
          <dd>Version {protocolVersion}</dd>
        </div>
      </dl>
    </section>
  );
}

function TechnicalValues({
  label,
  values,
  empty,
}: {
  label: string;
  values: string[];
  empty: string;
}) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>
        {values.length > 0 ? (
          <ul className={styles.technicalValues}>
            {values.map((value) => (
              <li key={value}>{value}</li>
            ))}
          </ul>
        ) : (
          <span className={styles.notDeclared}>{empty}</span>
        )}
      </dd>
    </div>
  );
}

function Notice({ children, id }: { children: string; id?: string }) {
  return (
    <output className={styles.notice} id={id} aria-live="polite">
      {children}
    </output>
  );
}

async function decide(
  review: Review,
  decision: Decision,
  setStage: (stage: Stage) => void,
  setNotice: (notice: string) => void,
) {
  setNotice("");
  setStage({ kind: "deciding", review, decision });
  const { source } = review;
  const isAuthorization = source.kind === "authorization";
  const action = decision === "approve" ? "approve" : "deny";
  const endpoint = isAuthorization
    ? `/api/v1/link/authorizations/${encodeURIComponent(source.requestCode)}/${action}`
    : `/api/v1/link/requests/${encodeURIComponent(source.userCode)}/${action}`;
  const options: RequestInit = {
    method: "POST",
    credentials: "same-origin",
    headers: isAuthorization
      ? BROWSER_MUTATION_HEADER
      : {
          ...BROWSER_MUTATION_HEADER,
          "Content-Type": "application/json",
        },
    body: isAuthorization
      ? undefined
      : JSON.stringify({ approvalToken: source.approvalToken }),
  };

  try {
    const response = await fetch(endpoint, options);
    const answer: unknown = await readJSON(response);
    if (!response.ok) {
      setNotice(
        errorMessage(
          answer,
          decision === "approve"
            ? "This link request could not be approved."
            : "This link request could not be declined.",
        ),
      );
      setStage({ kind: "confirm", review });
      return;
    }

    if (isAuthorization) {
      if (
        !isLinkRedirect(answer) ||
        !isSafeLoopbackRedirect(answer.redirectUrl)
      ) {
        setNotice(
          "Illarin returned an unsafe callback address. Nothing was opened.",
        );
        setStage({ kind: "confirm", review });
        return;
      }
      setStage({ kind: "redirecting", review, decision });
      window.location.assign(answer.redirectUrl);
      return;
    }

    if (decision === "deny") {
      setStage({ kind: "denied", link: review.link });
      return;
    }

    if (!isPendingLink(answer)) {
      setNotice("Illarin returned an incomplete approval. Try again.");
      setStage({ kind: "confirm", review });
      return;
    }
    setStage({ kind: "approved", link: answer });
  } catch {
    setNotice(UNREACHABLE);
    setStage({ kind: "confirm", review });
  }
}

async function readJSON(response: Response): Promise<unknown> {
  if (response.status === 204) return null;
  try {
    return await response.json();
  } catch {
    return null;
  }
}

function errorMessage(value: unknown, fallback: string) {
  if (
    typeof value === "object" &&
    value !== null &&
    "error" in value &&
    typeof value.error === "string" &&
    value.error.trim()
  ) {
    return value.error;
  }
  return fallback;
}

function isPendingLink(value: unknown): value is PendingLink {
  if (typeof value !== "object" || value === null) return false;
  const link = value as Record<string, unknown>;
  return (
    typeof link.applicationName === "string" &&
    typeof link.instanceName === "string" &&
    (link.applicationVersion === undefined ||
      link.applicationVersion === null ||
      typeof link.applicationVersion === "string") &&
    typeof link.protocolVersion === "number" &&
    isStringArray(link.capabilities) &&
    isStringArray(link.acceptedTargets) &&
    isStringArray(link.scopes) &&
    typeof link.expiresAt === "string"
  );
}

function isPendingDeviceLink(value: unknown): value is PendingDeviceLink {
  return (
    isPendingLink(value) &&
    "approvalToken" in value &&
    typeof value.approvalToken === "string"
  );
}

function isLinkRedirect(value: unknown): value is LinkRedirect {
  return (
    typeof value === "object" &&
    value !== null &&
    "redirectUrl" in value &&
    typeof value.redirectUrl === "string"
  );
}

function isStringArray(value: unknown): value is string[] {
  return (
    Array.isArray(value) && value.every((item) => typeof item === "string")
  );
}

function isSafeLoopbackRedirect(value: string) {
  const authority =
    /^http:\/\/(?:127\.0\.0\.1|\[::1\]):([0-9]{1,5})(?:[/?#]|$)/i.exec(value);
  if (!authority) return false;
  const port = Number(authority[1]);
  if (!Number.isInteger(port) || port < 1 || port > 65_535) return false;

  try {
    const url = new URL(value);
    return (
      url.protocol === "http:" &&
      (url.hostname === "127.0.0.1" || url.hostname === "[::1]") &&
      !url.username &&
      !url.password &&
      !url.hash
    );
  } catch {
    return false;
  }
}

function formatExpiry(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return value;
  return new Intl.DateTimeFormat("en-GB", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}
