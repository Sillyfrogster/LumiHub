"use client";

import { Eye, EyeOff, LockKeyhole } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import type { AssetDetail } from "@/lib/api/query";
import { saveAssetDiscovery } from "@/lib/api/query";
import { useAuth } from "@/lib/auth";
import styles from "./DiscoveryControl.module.css";

export function DiscoveryControl({
  assetId,
  creator,
  initialDiscovery,
  frozen,
}: {
  assetId: string;
  creator: string;
  initialDiscovery: AssetDetail["discovery"];
  frozen: boolean;
}) {
  const router = useRouter();
  const { account } = useAuth();
  const [discovery, setDiscovery] = useState(initialDiscovery);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  if (!account || account.handle !== creator) return null;

  const listed = discovery === "listed";
  const next = listed ? "unlisted" : "listed";

  async function changeDiscovery() {
    setPending(true);
    setMessage("");
    setDiscovery(next);
    try {
      await saveAssetDiscovery(assetId, next);
      router.refresh();
    } catch {
      setDiscovery(discovery);
      setMessage("Discovery could not be changed. Try again.");
    } finally {
      setPending(false);
    }
  }

  return (
    <section className={styles.control} aria-labelledby="discovery-heading">
      <div className={styles.icon} aria-hidden="true">
        {frozen ? (
          <LockKeyhole size={18} />
        ) : listed ? (
          <Eye size={18} />
        ) : (
          <EyeOff size={18} />
        )}
      </div>
      <div className={styles.copy}>
        <h2 id="discovery-heading">Catalog discovery</h2>
        <p>
          {frozen
            ? "Locked while this asset is withheld. Only staff can remove the withhold."
            : listed
              ? "Listed in the catalog and on your public profile."
              : "Unlisted from discovery. Anyone with the link can still view and download it."}
        </p>
        {message ? (
          <p className={styles.error} role="alert">
            {message}
          </p>
        ) : null}
      </div>
      <button
        type="button"
        onClick={changeDiscovery}
        disabled={pending || frozen}
      >
        {frozen
          ? "Discovery locked"
          : pending
            ? "Saving…"
            : listed
              ? "Make unlisted"
              : "List in catalog"}
      </button>
    </section>
  );
}
