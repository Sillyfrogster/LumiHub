import Link from "next/link";
import type { ReactNode } from "react";
import styles from "./SectionHead.module.css";

type EyebrowProps = {
  children: ReactNode;
  className?: string;
};

/** Small uppercase label that introduces a section */
export function Eyebrow({ children, className }: EyebrowProps) {
  return (
    <p
      className={className ? `${styles.eyebrow} ${className}` : styles.eyebrow}
    >
      {children}
    </p>
  );
}

type SectionHeadProps = {
  eyebrow?: string;
  title: ReactNode;
  action?: { label: string; href: string };
};

export function SectionHead({ eyebrow, title, action }: SectionHeadProps) {
  return (
    <div className={styles.head}>
      <div>
        {eyebrow && <Eyebrow>{eyebrow}</Eyebrow>}
        <h2 className={styles.title}>{title}</h2>
      </div>
      {action && (
        <Link href={action.href} className={styles.action}>
          {action.label} →
        </Link>
      )}
    </div>
  );
}
