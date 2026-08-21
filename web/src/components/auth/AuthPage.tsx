import type { ReactNode } from "react";
import { Shell } from "@/components/layout/Shell";
import styles from "./AuthPage.module.css";

type AuthPageProps = {
  title: ReactNode;
  introduction: string;
  children: ReactNode;
};

export function AuthPage({ title, introduction, children }: AuthPageProps) {
  return (
    <section className={styles.page}>
      <Shell className={styles.layout}>
        <div className={styles.introduction}>
          <h1 className={styles.title}>{title}</h1>
          <p className={styles.lede}>{introduction}</p>
        </div>
        <div className={styles.formArea}>{children}</div>
      </Shell>
    </section>
  );
}
