"use client";

import { LockKeyhole, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { deleteAsset } from "@/lib/api/query";
import { useAuth } from "@/lib/auth";
import styles from "./DeleteControl.module.css";

export function DeleteControl({
  assetId,
  creator,
  frozen,
}: {
  assetId: string;
  creator: string;
  frozen: boolean;
}) {
  const router = useRouter();
  const { account } = useAuth();
  const [confirming, setConfirming] = useState(false);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  if (!account || account.handle !== creator) return null;

  async function remove() {
    if (pending || frozen) return;
    setPending(true);
    setMessage("");
    try {
      await deleteAsset(assetId);
      router.replace(`/@${creator}#deleted`);
      router.refresh();
    } catch {
      setMessage("The asset could not be deleted. Try again.");
      setPending(false);
    }
  }

  return (
    <section className={styles.control} aria-labelledby="delete-heading">
      <div className={styles.icon} aria-hidden="true">
        {frozen ? <LockKeyhole size={18} /> : <Trash2 size={18} />}
      </div>
      <div className={styles.copy}>
        <h2 id="delete-heading">Delete this asset</h2>
        <p>
          {frozen
            ? "A withheld asset cannot be deleted."
            : "Its page and downloads stop now. You can restore it from your profile for 30 days."}
        </p>
        {confirming && !frozen ? (
          <div className={styles.confirmation}>
            <p>Move this asset to Deleted?</p>
            <button type="button" onClick={remove} disabled={pending}>
              {pending ? "Deleting…" : "Yes, delete it"}
            </button>
            <button
              type="button"
              className={styles.cancel}
              onClick={() => setConfirming(false)}
              disabled={pending}
            >
              Keep it
            </button>
          </div>
        ) : null}
        {message ? (
          <p className={styles.error} role="alert">
            {message}
          </p>
        ) : null}
      </div>
      {!confirming ? (
        <button
          type="button"
          onClick={() => setConfirming(true)}
          disabled={frozen}
        >
          {frozen ? "Deletion locked" : "Move to Deleted"}
        </button>
      ) : null}
    </section>
  );
}
