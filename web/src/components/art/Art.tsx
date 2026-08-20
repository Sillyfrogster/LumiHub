import Image from "next/image";
import type { CSSProperties } from "react";
import { ART, type ArtName } from "@/lib/art.generated";
import styles from "./Art.module.css";

type ArtProps = {
  name: ArtName;
  /** Width used for layout and responsive image selection */
  width?: number;
  sizes?: string;
  className?: string;
  style?: CSSProperties;
  alt?: string;
  preload?: boolean;
  loading?: "eager";
};

export function Art({
  name,
  width,
  sizes,
  className,
  style,
  alt,
  preload,
  loading,
}: ArtProps) {
  return (
    <Image
      src={ART[name]}
      alt={alt ?? ""}
      aria-hidden={alt ? undefined : true}
      sizes={sizes ?? (width ? `${width}px` : "100vw")}
      preload={preload}
      loading={loading}
      className={className ? `${styles.art} ${className}` : styles.art}
      style={width ? { width, height: "auto", ...style } : style}
    />
  );
}
