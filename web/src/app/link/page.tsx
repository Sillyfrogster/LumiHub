import { Suspense } from "react";
import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import { LinkApproval } from "@/components/linking/LinkApproval";
import styles from "./LinkPage.module.css";

export const metadata = {
  title: "Link an application",
};

export default function LinkPage() {
  return (
    <section className={styles.page}>
      <Art name="wash" width={720} className={styles.wash} loading="eager" />
      <Art name="city" width={520} className={styles.city} />
      <Shell className={styles.layout}>
        <div className={styles.introduction}>
          <p className={styles.eyebrow}>Linking</p>
          <h1 className={styles.title}>
            Give one application <em>a way in.</em>
          </h1>
          <p className={styles.lede}>
            Approving a code links one installation to your account, whether it
            runs on this machine, your network, a server somewhere else, or a
            box with no screen at all.
          </p>
          <div className={styles.rule} aria-hidden="true">
            <span />
            <Art name="rule-flower" width={58} />
            <span />
          </div>
          <p className={styles.note}>
            Nothing is linked until you approve it, and you can cut any link
            afterwards from your settings.
          </p>
        </div>
        <div className={styles.panelArea}>
          <Suspense fallback={<p className={styles.opening}>Opening…</p>}>
            <LinkApproval />
          </Suspense>
        </div>
      </Shell>
      <Art name="sprig" width={260} className={styles.sprig} />
    </section>
  );
}
