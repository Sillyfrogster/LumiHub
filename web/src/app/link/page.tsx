import Image from "next/image";
import { Suspense } from "react";
import linkPassage from "@/assets/art/full/illarin-link-passage-v1.webp";
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
            <div className={styles.introColumn}>
              <header className={styles.introduction}>
                <h1 className={styles.title}>Link an application</h1>
                <p className={styles.lede}>
                  Approve a private connection between your Illarin account and
                  an application you opened.
                </p>
              </header>
              <p className={styles.note}>
                You can revoke a linked installation at any time from account
                settings.
              </p>
            </div>
            <div className={styles.panelArea}>
              <Suspense fallback={<p className={styles.opening}>Opening…</p>}>
                <LinkApproval />
              </Suspense>
            </div>
          </div>
          <div className={styles.artwork}>
            <Image
              src={linkPassage}
              alt="Two Illarin archive passages standing side by side"
              fill
              priority
              sizes="(max-width: 820px) 100vw, 980px"
            />
          </div>
        </div>
      </Shell>
    </section>
  );
}
