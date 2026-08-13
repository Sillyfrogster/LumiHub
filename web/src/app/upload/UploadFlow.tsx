"use client";

import { AlertCircle, Check, FileArchive, Upload } from "lucide-react";
import Link from "next/link";
import {
  type ChangeEvent,
  type FormEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import type { components } from "@/lib/api/schema";
import { useAuth } from "@/lib/auth";
import styles from "./UploadPage.module.css";

type IngestOperation = components["schemas"]["IngestOperation"];
type ErrorAnswer = { error?: string };

const KIND_LABELS = {
  character: "Character",
  lorebook: "Lorebook",
  preset: "Preset",
  theme: "Theme",
} as const;

function catalogName(filename: string) {
  const lastDot = filename.lastIndexOf(".");
  return lastDot > 0 ? filename.slice(0, lastDot) : filename;
}

function operationPath(url: string) {
  return `/api${url}`;
}

export function UploadFlow() {
  const { account } = useAuth();
  const fileInput = useRef<HTMLInputElement>(null);
  const operationHeading = useRef<HTMLHeadingElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState("");
  const [operation, setOperation] = useState<IngestOperation | null>(null);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (
      !operation ||
      (operation.status !== "pending" && operation.status !== "processing")
    ) {
      return;
    }

    const current = operation;
    const controller = new AbortController();
    let active = true;
    async function poll() {
      while (active) {
        await new Promise((resolve) => setTimeout(resolve, 600));
        if (!active) return;
        try {
          const response = await fetch(operationPath(current.url), {
            credentials: "same-origin",
            cache: "no-store",
            signal: controller.signal,
          });
          if (!response.ok) {
            const answer = (await response.json()) as ErrorAnswer;
            setMessage(
              answer.error ?? "LumiHub could not read this upload yet.",
            );
            return;
          }
          const next = (await response.json()) as IngestOperation;
          setOperation(next);
          if (next.status !== "pending" && next.status !== "processing") {
            return;
          }
        } catch (error) {
          if (!(error instanceof DOMException && error.name === "AbortError")) {
            setMessage(
              "The connection was interrupted. Your upload is safe; reload to check it again.",
            );
          }
          return;
        }
      }
    }
    void poll();
    return () => {
      active = false;
      controller.abort();
    };
  }, [operation]);

  useEffect(() => {
    if (
      operation &&
      operation.status !== "pending" &&
      operation.status !== "processing"
    ) {
      operationHeading.current?.focus();
    }
  }, [operation]);

  function chooseFile(event: ChangeEvent<HTMLInputElement>) {
    const chosen = event.target.files?.[0] ?? null;
    setFile(chosen);
    setName(chosen ? catalogName(chosen.name) : "");
    setMessage("");
  }

  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!file) {
      setMessage("Choose the original file before uploading.");
      fileInput.current?.focus();
      return;
    }

    const form = new FormData(event.currentTarget);
    const tags = String(form.get("tags") ?? "")
      .split(",")
      .map((tag) => tag.trim())
      .filter((tag, index, all) => tag && all.indexOf(tag) === index);
    const body = new FormData();
    const metadata: Record<string, unknown> = {
      confirmed: form.get("confirmed") === "on",
      name: String(form.get("name") ?? "").trim(),
      discovery: String(form.get("discovery") ?? "listed"),
    };
    const blurb = String(form.get("blurb") ?? "").trim();
    if (blurb) metadata.blurb = blurb;
    if (tags.length) metadata.tags = tags;
    if (form.get("isNsfw") === "on") metadata.isNsfw = true;
    body.append("metadata", JSON.stringify(metadata));
    body.append("file", file, file.name);

    await requestOperation(
      "/api/v1/assets",
      {
        method: "POST",
        credentials: "same-origin",
        body,
      },
      "LumiHub could not accept this upload.",
    );
  }

  async function completeKind(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!operation) return;
    const form = new FormData(event.currentTarget);
    await requestOperation(
      operationPath(operation.url),
      {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({
          kind: String(form.get("kind") ?? ""),
          name: String(form.get("name") ?? "").trim(),
        }),
      },
      "LumiHub could not save those details.",
    );
  }

  async function requestOperation(
    path: string,
    init: RequestInit,
    refusal: string,
  ) {
    setPending(true);
    setMessage("");
    try {
      const response = await fetch(path, init);
      const answer = (await response.json()) as IngestOperation & ErrorAnswer;
      if (!response.ok) {
        setMessage(answer.error ?? refusal);
        return;
      }
      setOperation(answer);
    } catch {
      setMessage(
        "LumiHub could not be reached. Check your connection and try again.",
      );
    } finally {
      setPending(false);
    }
  }

  function beginAgain() {
    setOperation(null);
    setFile(null);
    setName("");
    setMessage("");
    if (fileInput.current) fileInput.current.value = "";
  }

  function retryPoll() {
    setMessage("");
    setOperation((current) => (current ? { ...current } : current));
  }

  if (account === undefined) {
    return <p className={styles.sessionState}>Reading your account…</p>;
  }

  if (!account) {
    return (
      <section className={styles.accountState}>
        <FileArchive size={28} strokeWidth={1.35} aria-hidden="true" />
        <h2>Sign in before uploading</h2>
        <p>Your account keeps every creation tied to the right creator.</p>
        <Link href="/sign-in">Sign in</Link>
      </section>
    );
  }

  if (!account.emailVerified) {
    return (
      <section className={styles.accountState}>
        <FileArchive size={28} strokeWidth={1.35} aria-hidden="true" />
        <h2>Verify your email before uploading</h2>
        <p>
          Verification puts every public file behind an address you control.
        </p>
        <Link href="/verify-email">Verify email</Link>
      </section>
    );
  }

  if (operation?.status === "needs_kind" && operation.needsKind) {
    return (
      <section className={styles.operation} aria-labelledby="kind-heading">
        <div className={styles.operationIcon}>
          <FileArchive size={28} strokeWidth={1.3} aria-hidden="true" />
        </div>
        <div className={styles.operationCopy}>
          <h2 ref={operationHeading} id="kind-heading" tabIndex={-1}>
            Tell us where this belongs
          </h2>
          <p>
            The file is safe to keep, but its format does not say what kind of
            creation it holds.
          </p>
        </div>
        <form className={styles.kindForm} onSubmit={completeKind}>
          <label>
            Kind
            <select name="kind" required defaultValue="">
              <option value="" disabled>
                Choose a kind
              </option>
              {Object.entries(KIND_LABELS).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Catalog name
            <input
              name="name"
              required
              defaultValue={operation.needsKind.name}
              autoComplete="off"
            />
          </label>
          <button type="submit" disabled={pending}>
            {pending ? "Saving…" : "Finish ingest"}
          </button>
        </form>
        {message ? (
          <p className={styles.error} role="alert">
            {message}
          </p>
        ) : null}
      </section>
    );
  }

  if (operation?.status === "failed" && operation.failure) {
    return (
      <section className={styles.operation} aria-labelledby="failed-heading">
        <div className={`${styles.operationIcon} ${styles.failedIcon}`}>
          <AlertCircle size={28} strokeWidth={1.35} aria-hidden="true" />
        </div>
        <div className={styles.operationCopy}>
          <h2 ref={operationHeading} id="failed-heading" tabIndex={-1}>
            This file was not added
          </h2>
          <p>{operation.failure.message}</p>
        </div>
        <button className={styles.secondary} type="button" onClick={beginAgain}>
          Choose another file
        </button>
      </section>
    );
  }

  if (operation?.status === "success" && operation.asset) {
    const created = operation.asset;
    return (
      <section className={styles.success} aria-labelledby="success-heading">
        <div className={styles.successMark}>
          <Check size={30} strokeWidth={1.5} aria-hidden="true" />
        </div>
        <p className={styles.kind}>
          {KIND_LABELS[created.kind as keyof typeof KIND_LABELS] ??
            created.kind}
        </p>
        <h2 ref={operationHeading} id="success-heading" tabIndex={-1}>
          {created.name}
        </h2>
        {created.blurb ? (
          <p className={styles.createdBlurb}>{created.blurb}</p>
        ) : null}
        <div className={styles.successActions}>
          <a href={`/api/v1/assets/${created.id}/original`}>
            Download original
          </a>
          <button type="button" onClick={beginAgain}>
            Upload another
          </button>
        </div>
      </section>
    );
  }

  if (operation) {
    if (message) {
      return (
        <section className={styles.processing} aria-live="polite">
          <AlertCircle
            className={styles.processingErrorIcon}
            size={31}
            strokeWidth={1.35}
            aria-hidden="true"
          />
          <h2>We lost the latest update</h2>
          <p>{message}</p>
          <button
            className={styles.secondary}
            type="button"
            onClick={retryPoll}
          >
            Check again
          </button>
        </section>
      );
    }
    return (
      <section className={styles.processing} aria-live="polite">
        <FileArchive size={31} strokeWidth={1.35} aria-hidden="true" />
        <h2>
          {operation.status === "pending"
            ? "Your file is in hand"
            : "Reading your file"}
        </h2>
        <p>
          You can leave this page. Ingest will continue and this page will keep
          checking for the result while it remains open.
        </p>
      </section>
    );
  }

  return (
    <form className={styles.form} onSubmit={upload}>
      <div className={styles.fileField}>
        <input
          ref={fileInput}
          id="asset-file"
          name="file"
          type="file"
          required
          onChange={chooseFile}
        />
        <label htmlFor="asset-file">
          <Upload size={25} strokeWidth={1.35} aria-hidden="true" />
          <span>{file ? file.name : "Choose the original file"}</span>
          <small>
            {file ? "Choose a different file" : "Any safe format up to 32 MB"}
          </small>
        </label>
      </div>

      <div className={styles.catalogFields}>
        <label className={styles.wideField}>
          Catalog name
          <input
            name="name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            required
            autoComplete="off"
            placeholder="What readers will call it"
          />
        </label>
        <label className={styles.wideField}>
          Blurb
          <textarea
            name="blurb"
            rows={4}
            placeholder="A short invitation written for a person"
          />
        </label>
        <label>
          Tags
          <input
            name="tags"
            autoComplete="off"
            placeholder="fantasy, cozy, mystery"
          />
          <small>Separate free-text tags with commas.</small>
        </label>
        <label>
          Discovery
          <select name="discovery" defaultValue="listed">
            <option value="listed">Listed in the catalog</option>
            <option value="unlisted">Unlisted, reachable by its link</option>
          </select>
        </label>
      </div>

      <div className={styles.checks}>
        <label>
          <input name="isNsfw" type="checkbox" />
          <span>Mark this catalog entry as NSFW</span>
        </label>
        <label>
          <input name="confirmed" type="checkbox" required />
          <span>I have checked the name, blurb, tags and NSFW flag.</span>
        </label>
      </div>

      {message ? (
        <p className={styles.error} role="alert">
          {message}
        </p>
      ) : null}

      <button
        className={styles.submit}
        type="submit"
        disabled={pending || !file}
      >
        {pending ? "Handing over the file…" : "Begin ingest"}
      </button>
    </form>
  );
}
