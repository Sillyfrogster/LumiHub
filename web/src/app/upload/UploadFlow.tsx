"use client";

import { AlertCircle, Check, FileArchive, Upload } from "lucide-react";
import Link from "next/link";
import {
  type ChangeEvent,
  type FormEvent,
  type RefObject,
  useEffect,
  useRef,
  useState,
} from "react";
import {
  fetchPreservedNamespaces,
  type PreservedNamespace,
} from "@/lib/api/query";
import type { components } from "@/lib/api/schema";
import { assetHref } from "@/lib/asset-url";
import { useAuth } from "@/lib/auth";
import { describePreservedNamespaces } from "@/lib/preserved";
import { StartFromNothing } from "./StartFromNothing";
import styles from "./UploadPage.module.css";

type IngestOperation = components["schemas"]["IngestOperation"];
type CreatedAsset = NonNullable<IngestOperation["asset"]>;
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
  const [preserved, setPreserved] = useState<PreservedNamespace[] | null>(null);
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

  const importedAssetId =
    operation?.status === "success" && operation.asset
      ? operation.asset.id
      : null;

  useEffect(() => {
    if (!importedAssetId) return;
    let active = true;
    setPreserved(null);
    void fetchPreservedNamespaces(importedAssetId).then((found) => {
      if (active) setPreserved(found);
    });
    return () => {
      active = false;
    };
  }, [importedAssetId]);

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
    setPreserved(null);
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
    return (
      <ImportReceipt
        asset={operation.asset}
        preserved={preserved}
        headingRef={operationHeading}
        onBeginAgain={beginAgain}
      />
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
    <div className={styles.flow}>
      <form className={styles.form} onSubmit={upload}>
        <div className={styles.formIntro}>
          <div>
            <h2>Start with an original file</h2>
            <p>Choose the file, then give its catalog entry a name.</p>
          </div>
        </div>

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
              {file
                ? "Choose a different file"
                : "Original file, within the upload limit"}
            </small>
          </label>
        </div>

        <div className={styles.catalogFields}>
          <label>
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
          <label>
            Discovery
            <select name="discovery" defaultValue="listed">
              <option value="listed">Listed in the catalog</option>
              <option value="unlisted">Unlisted, reachable by its link</option>
            </select>
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
      <StartFromNothing />
    </div>
  );
}

function ImportReceipt({
  asset,
  preserved,
  headingRef,
  onBeginAgain,
}: {
  asset: CreatedAsset;
  preserved: PreservedNamespace[] | null;
  headingRef: RefObject<HTMLHeadingElement | null>;
  onBeginAgain: () => void;
}) {
  const kind =
    KIND_LABELS[asset.kind as keyof typeof KIND_LABELS] ?? asset.kind;
  const carriedDescription =
    preserved && preserved.length > 0
      ? describePreservedNamespaces(preserved.map(({ name }) => name))
      : null;

  return (
    <section className={styles.success} aria-labelledby="success-heading">
      <div className={styles.receiptHeader}>
        <div className={styles.successMark}>
          <Check size={30} strokeWidth={1.5} aria-hidden="true" />
        </div>
        <div>
          <h2 ref={headingRef} id="success-heading" tabIndex={-1}>
            Your file is ready to shape
          </h2>
          <p className={styles.receiptName}>{asset.name}</p>
        </div>
      </div>

      <div className={styles.receipt}>
        <h3>What came across</h3>
        <p className={styles.receiptLead}>
          Illarin read the parts it knows how to edit. The rest stays with your
          original and travels back out in downloads for its format.
        </p>
        {preserved === null ? (
          <p className={styles.receiptPending}>
            Reading what your file carried…
          </p>
        ) : carriedDescription ? (
          <p className={styles.carriedSummary}>
            Your file also carried {carriedDescription}. Illarin keeps those
            details with the original and sends them back out in compatible
            downloads.
          </p>
        ) : (
          <p className={styles.receiptClear}>
            Everything your file carried is ready to edit. Nothing extra needs
            to be kept.
          </p>
        )}
        <p className={styles.receiptNote}>
          You can manage carried data later from Creator tools on the page.
        </p>
      </div>

      <div className={styles.successActions}>
        <Link
          className={styles.openPage}
          href={assetHref(asset.id, asset.name)}
        >
          Open the {kind.toLowerCase()} page
        </Link>
        <button type="button" onClick={onBeginAgain}>
          Upload another
        </button>
      </div>
    </section>
  );
}
