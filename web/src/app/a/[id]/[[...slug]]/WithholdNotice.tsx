import { LockKeyhole } from "lucide-react";
import type { AssetDetail } from "@/lib/api/query";
import styles from "./WithholdNotice.module.css";

export function WithholdNotice({
  withhold,
}: {
  withhold: NonNullable<AssetDetail["withhold"]>;
}) {
  const recorded = new Date(withhold.at).toLocaleString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });

  return (
    <section className={styles.notice} aria-labelledby="withhold-heading">
      <LockKeyhole size={19} aria-hidden="true" />
      <div>
        <h2 id="withhold-heading">Withheld from public view</h2>
        <p className={styles.reason}>{withhold.reason}</p>
        <p className={styles.recorded}>
          Recorded by @{withhold.actor} on {recorded}
        </p>
      </div>
    </section>
  );
}
