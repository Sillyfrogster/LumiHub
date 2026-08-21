"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import { saveAssetIdentity } from "@/lib/api/query";
import styles from "./IdentityPanel.module.css";

/** The three states the adult content question has while an asset is a draft. */
const ANSWERS: { value: boolean | null; label: string }[] = [
  { value: null, label: "Not yet" },
  { value: false, label: "No" },
  { value: true, label: "Yes" },
];

export function IdentityPanel({
  assetId,
  initialName,
  initialIsNsfw,
  isDraft,
}: {
  assetId: string;
  initialName: string;
  initialIsNsfw: boolean | null;
  isDraft: boolean;
}) {
  const router = useRouter();
  const [name, setName] = useState(initialName);
  const [isNsfw, setIsNsfw] = useState(initialIsNsfw);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [saved, setSaved] = useState(false);

  const answers = isDraft ? ANSWERS : ANSWERS.filter((a) => a.value !== null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setPending(true);
    setMessage("");
    setSaved(false);
    try {
      await saveAssetIdentity(assetId, { name, isNsfw });
      setSaved(true);
      router.refresh();
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "The details could not be saved. Try again.",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <section className={styles.panel} aria-labelledby="identity-heading">
      <h2 id="identity-heading">Name and rating</h2>
      <form onSubmit={submit}>
        <label htmlFor="asset-name">Name</label>
        <input
          id="asset-name"
          value={name}
          placeholder="Name this page"
          onChange={(event) => {
            setSaved(false);
            setName(event.target.value);
          }}
          disabled={pending}
        />

        <fieldset id="adult-content-answer" className={styles.rating}>
          <legend>Adult content</legend>
          <div className={styles.answers}>
            {answers.map((answer) => (
              <button
                key={answer.label}
                type="button"
                aria-pressed={answer.value === isNsfw}
                onClick={() => {
                  setSaved(false);
                  setIsNsfw(answer.value);
                }}
                disabled={pending}
              >
                {answer.label}
              </button>
            ))}
          </div>
        </fieldset>
        <p className={styles.note}>
          {isNsfw === null
            ? "Publishing will not go through until this is answered. There is no default."
            : "You can change this after publishing."}
        </p>

        <button type="submit" className={styles.save} disabled={pending}>
          {pending ? "Saving…" : "Save"}
        </button>
        {saved ? <p className={styles.saved}>Saved.</p> : null}
        {message ? (
          <p className={styles.error} role="alert">
            {message}
          </p>
        ) : null}
      </form>
    </section>
  );
}
