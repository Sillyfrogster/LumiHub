import Link from "next/link";
import type { ReactNode } from "react";
import styles from "./Button.module.css";

type ButtonProps = {
  children: ReactNode;
  href?: string;
  variant?: "solid" | "outline";
  size?: "small" | "medium" | "large";
  block?: boolean;
  className?: string;
};

const SIZE = {
  small: styles.small,
  medium: styles.medium,
  large: styles.large,
} as const;

export function Button({
  children,
  href,
  variant = "solid",
  size = "medium",
  block,
  className,
}: ButtonProps) {
  const classes = [
    styles.button,
    styles[variant],
    SIZE[size],
    block ? styles.block : "",
    className ?? "",
  ]
    .filter(Boolean)
    .join(" ");

  if (href) {
    return (
      <Link href={href} className={classes}>
        {children}
      </Link>
    );
  }

  return (
    <button type="button" className={classes}>
      {children}
    </button>
  );
}
