"use client";

import { Shield } from "lucide-react";
import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import { withholdAsset } from "@/lib/api/query";
import { useAuth } from "@/lib/auth";
import styles from "./WithholdControl.module.css";

export function WithholdControl({ assetId }: { assetId: string }) {
  const router = useRouter();
  const { account } = useAuth();
  const [reason, setReason] = useState("");
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  if (account?.role !== "admin") return null;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const decision = reason.trim();
    if (!decision || pending) return;
    setPending(true);
    setMessage("");
    try {
      await withholdAsset(assetId, decision);
      router.replace("/browse");
    } catch {
      setMessage("The asset could not be withheld. Try again.");
      setPending(false);
    }
  }

  return (
    <section
      className={styles.control}
      aria-labelledby="withhold-control-heading"
    >
      <div className={styles.heading}>
        <Shield size={18} aria-hidden="true" />
        <div>
          <h2 id="withhold-control-heading">Staff action</h2>
          <p>Remove public access without deleting the creator’s file.</p>
        </div>
      </div>
      <form onSubmit={submit}>
        <label htmlFor="withhold-reason">Reason shown to the creator</label>
        <textarea
          id="withhold-reason"
          rows={3}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          disabled={pending}
          required
        />
        <button type="submit" disabled={pending || !reason.trim()}>
          {pending ? "Withholding…" : "Withhold asset"}
        </button>
        {message ? (
          <p className={styles.error} role="alert">
            {message}
          </p>
        ) : null}
      </form>
    </section>
  );
}
