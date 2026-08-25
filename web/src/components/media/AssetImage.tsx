import Image from "next/image";
import type { ReactNode } from "react";
import styles from "./AssetImage.module.css";

type AssetImageProps = {
  src?: string;
  alt: string;
  className?: string;
  children?: ReactNode;
};

/**
 * Artwork for a creation. Falls back to an empty frame when a creation has no
 * cover, which keeps grids from collapsing.
 */
export function AssetImage({ src, alt, className, children }: AssetImageProps) {
  return (
    <div
      className={className ? `${styles.frame} ${className}` : styles.frame}
      data-empty={src ? undefined : ""}
    >
      {src && (
        <Image
          src={src}
          alt={alt}
          fill
          sizes="360px"
          className={styles.image}
        />
      )}
      {children}
    </div>
  );
}
