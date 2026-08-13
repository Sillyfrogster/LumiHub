import type { ReactNode } from "react";
import { Art } from "@/components/art/Art";
import { Shell } from "@/components/layout/Shell";
import styles from "./AuthPage.module.css";

type AuthPageProps = {
  eyebrow: string;
  title: ReactNode;
  introduction: string;
  children: ReactNode;
};

export function AuthPage({
  eyebrow,
  title,
  introduction,
  children,
}: AuthPageProps) {
  return (
    <section className={styles.page}>
      <Art name="wash" width={760} className={styles.wash} />
      <Art name="birds" width={210} className={styles.birds} />
      <Shell className={styles.layout}>
        <div className={styles.introduction}>
          <p className={styles.eyebrow}>{eyebrow}</p>
          <h1 className={styles.title}>{title}</h1>
          <p className={styles.lede}>{introduction}</p>
          <div className={styles.rule} aria-hidden="true">
            <span />
            <Art name="rule-flower" width={62} />
            <span />
          </div>
          <p className={styles.note}>
            Your handle is the address of everything you make here. Choose one
            you will be glad to sign.
          </p>
        </div>
        <div className={styles.formArea}>{children}</div>
      </Shell>
      <Art name="sprig" width={280} className={styles.sprig} priority />
    </section>
  );
}
