"use client";

import { Trash2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  deletePreservedNamespace,
  fetchPreservedNamespaces,
  type PreservedNamespace,
} from "@/lib/api/query";
import styles from "./PreservedPanel.module.css";

/** How much a namespace holds, in the units a person reads. */
function sizeLabel(bytes: number): string {
  if (bytes < 1024) return `${bytes} bytes`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * What the file carried that Illarin does not understand. The panel names it
 * and deletes it, and never offers to edit it: there is no way to check JSON
 * Illarin cannot read, so an editor here would be a foot-gun with no upside.
 *
 * It is absent rather than empty when the asset carries nothing.
 */
export function PreservedPanel({ assetId }: { assetId: string }) {
  const [namespaces, setNamespaces] = useState<PreservedNamespace[]>([]);
  const [deleting, setDeleting] = useState<PreservedNamespace | null>(null);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    let current = true;
    void fetchPreservedNamespaces(assetId).then((found) => {
      if (current) setNamespaces(found);
    });
    return () => {
      current = false;
    };
  }, [assetId]);

  if (namespaces.length === 0) return null;

  async function remove(namespace: string) {
    if (pending) return;
    setPending(true);
    setMessage("");
    try {
      await deletePreservedNamespace(assetId, namespace);
      setNamespaces((current) =>
        current.filter((held) => held.name !== namespace),
      );
      setDeleting(null);
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "That data could not be deleted. Try again.",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <section className={styles.panel} aria-labelledby="preserved-heading">
      <h2 id="preserved-heading">Carried from your file</h2>
      <p className={styles.lead}>
        Your file brought data Illarin does not read. It is kept exactly as it
        arrived and goes back into your downloads untouched.
      </p>
      <ul className={styles.namespaces}>
        {namespaces.map((namespace) => (
          <li key={namespace.name}>
            <span>
              <strong>{namespace.name}</strong>
              <span className={styles.size}>{sizeLabel(namespace.bytes)}</span>
            </span>
            <button
              type="button"
              onClick={() => {
                setMessage("");
                setDeleting(namespace);
              }}
            >
              <Trash2 size={15} aria-hidden="true" />
              <span className={styles.buttonLabel}>
                Delete {namespace.name}
              </span>
            </button>
          </li>
        ))}
      </ul>
      {message ? (
        <p className={styles.error} role="alert">
          {message}
        </p>
      ) : null}
      {deleting ? (
        <DeleteNamespaceDialog
          namespace={deleting}
          pending={pending}
          onCancel={() => setDeleting(null)}
          onDelete={() => void remove(deleting.name)}
        />
      ) : null}
    </section>
  );
}

/** The same confirmation removing a section gets, for the same reason. */
function DeleteNamespaceDialog({
  namespace,
  pending,
  onCancel,
  onDelete,
}: {
  namespace: PreservedNamespace;
  pending: boolean;
  onCancel: () => void;
  onDelete: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => dialog.current?.showModal(), []);

  return (
    <dialog
      ref={dialog}
      className={styles.dialog}
      onCancel={(event) => {
        event.preventDefault();
        onCancel();
      }}
    >
      <div className={styles.dialogBody}>
        <p className={styles.context}>Delete carried data</p>
        <h2>Delete “{namespace.name}”?</h2>
        <p>
          This deletes {sizeLabel(namespace.bytes)} of data your file carried
          under <strong>{namespace.name}</strong>. It will stop travelling in
          your downloads.
        </p>
        <p className={styles.noCopy}>
          There is nowhere else this is kept. Deleting it is permanent, and
          re-uploading the original file is the only way back.
        </p>
      </div>
      <footer className={styles.dialogFooter}>
        <button type="button" onClick={onCancel} disabled={pending}>
          Cancel
        </button>
        <button
          type="button"
          className={styles.confirmDelete}
          onClick={onDelete}
          disabled={pending}
        >
          <Trash2 size={17} aria-hidden="true" />
          {pending ? "Deleting…" : "Delete permanently"}
        </button>
      </footer>
    </dialog>
  );
}
