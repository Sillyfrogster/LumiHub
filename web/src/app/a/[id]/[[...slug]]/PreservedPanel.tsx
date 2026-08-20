"use client";

import { ChevronRight, RotateCcw, Trash2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  deletePreservedNamespace,
  fetchPreservedNamespaces,
  type PreservedNamespace,
} from "@/lib/api/query";
import { describePreservedNamespace } from "@/lib/preserved";
import styles from "./PreservedPanel.module.css";

/**
 * The source file's unread remainder is an owner tool, not page content. It
 * stays closed until a creator asks for it.
 */
export function PreservedPanel({ assetId }: { assetId: string }) {
  const [open, setOpen] = useState(false);
  const [namespaces, setNamespaces] = useState<PreservedNamespace[] | null>(
    null,
  );
  const [deleting, setDeleting] = useState<PreservedNamespace | null>(null);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  async function openMenu() {
    setOpen(true);
    if (namespaces !== null) return;
    setMessage("");
    const found = await fetchPreservedNamespaces(assetId);
    setNamespaces(found);
  }

  async function remove(namespace: string) {
    if (pending) return;
    setPending(true);
    setMessage("");
    try {
      await deletePreservedNamespace(assetId, namespace);
      setNamespaces(
        (current) =>
          current?.filter((held) => held.name !== namespace) ?? current,
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
    <div className={styles.control}>
      <button
        className={styles.launch}
        type="button"
        aria-expanded={open}
        aria-controls="preserved-menu"
        onClick={() => {
          if (open) {
            setOpen(false);
          } else {
            void openMenu();
          }
        }}
      >
        <span className={styles.launchCopy}>
          <strong>Manage file extras</strong>
          <span>Review what your upload kept for compatible downloads</span>
        </span>
        <ChevronRight
          className={open ? styles.chevronOpen : undefined}
          size={18}
          aria-hidden="true"
        />
      </button>

      {open ? (
        <div className={styles.menu} id="preserved-menu">
          <p className={styles.menuLead}>
            These details came with the original file but are not part of the
            editable page. Removing one stops it travelling in downloads for
            that format.
          </p>
          {namespaces === null ? (
            <p className={styles.menuState}>Reading the file extras…</p>
          ) : namespaces.length === 0 ? (
            <p className={styles.menuState}>
              Nothing extra is being kept with this upload.
            </p>
          ) : (
            <ul className={styles.namespaces}>
              {namespaces.map((namespace) => (
                <li key={namespace.name}>
                  <span>{describePreservedNamespace(namespace.name)}</span>
                  <button
                    className={styles.remove}
                    type="button"
                    onClick={() => {
                      setMessage("");
                      setDeleting(namespace);
                    }}
                  >
                    <Trash2 size={15} aria-hidden="true" />
                    <span className="sr-only">
                      Remove {describePreservedNamespace(namespace.name)}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
          {message ? (
            <p className={styles.error} role="alert">
              {message}
            </p>
          ) : null}
          {namespaces === null ? (
            <button
              className={styles.retry}
              type="button"
              onClick={() => {
                setNamespaces(null);
                void openMenu();
              }}
            >
              <RotateCcw size={14} aria-hidden="true" />
              Try again
            </button>
          ) : null}
        </div>
      ) : null}

      {deleting ? (
        <DeleteNamespaceDialog
          namespace={deleting}
          pending={pending}
          onCancel={() => setDeleting(null)}
          onDelete={() => void remove(deleting.name)}
        />
      ) : null}
    </div>
  );
}

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
  const description = describePreservedNamespace(namespace.name);

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
        <p className={styles.context}>Remove file extras</p>
        <h2>Remove {description}?</h2>
        <p>
          This detail came with your original file. Removing it means it will
          stop travelling in downloads made for that format.
        </p>
        <p className={styles.noCopy}>
          This cannot be undone here. Re-upload the original file if you need it
          back.
        </p>
      </div>
      <footer className={styles.dialogFooter}>
        <button type="button" onClick={onCancel} disabled={pending}>
          Keep it
        </button>
        <button
          type="button"
          className={styles.confirmDelete}
          onClick={onDelete}
          disabled={pending}
        >
          <Trash2 size={17} aria-hidden="true" />
          {pending ? "Removing…" : "Remove permanently"}
        </button>
      </footer>
    </dialog>
  );
}
