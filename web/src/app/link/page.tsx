import { Suspense } from "react";
import { Shell } from "@/components/layout/Shell";
import { LinkApproval } from "@/components/linking/LinkApproval";
import styles from "./LinkPage.module.css";

export const metadata = {
  title: "Link an application",
};

export default function LinkPage() {
  return (
    <section className={styles.page}>
      <Shell className={styles.layout}>
        <div className={styles.desk}>
          <div className={styles.task}>
            <Suspense fallback={<p className={styles.opening}>Opening…</p>}>
              <LinkApproval />
            </Suspense>
            <p className={styles.note}>
              You can revoke a linked installation at any time from account
              settings.
            </p>
          </div>
          <div className={styles.artwork} aria-hidden="true" />
        </div>
      </Shell>
    </section>
  );
}
