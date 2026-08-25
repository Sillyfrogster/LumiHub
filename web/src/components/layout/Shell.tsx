import type { ElementType, ReactNode } from "react";
import styles from "./Shell.module.css";

type ShellProps = {
  children: ReactNode;
  as?: ElementType;
  className?: string;
};

/** Holds page content to a fixed width */
export function Shell({ children, as: Tag = "div", className }: ShellProps) {
  return (
    <Tag className={className ? `${styles.shell} ${className}` : styles.shell}>
      {children}
    </Tag>
  );
}
