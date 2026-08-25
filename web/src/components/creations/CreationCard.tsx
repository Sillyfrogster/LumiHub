import { Download, Heart, MessageCircle, Star } from "lucide-react";
import Link from "next/link";
import { AssetImage } from "@/components/media/AssetImage";
import { formatCount } from "@/lib/format";
import { creationHref } from "@/lib/routes";
import { type Creation, KIND_LABEL } from "@/types/creation";
import styles from "./CreationCard.module.css";

export type CreationMetric =
  | "rating"
  | "downloads"
  | "favorites"
  | "comments"
  | "frontends";

type CreationCardProps = {
  creation: Creation;
  metrics?: CreationMetric[];
  size?: "small" | "large";
};

const DEFAULT_METRICS: CreationMetric[] = ["rating", "downloads", "favorites"];

const ICON = { size: 10, strokeWidth: 1.2 };

export function CreationCard({
  creation,
  metrics = DEFAULT_METRICS,
  size = "small",
}: CreationCardProps) {
  return (
    <Link
      href={creationHref(creation)}
      className={`${styles.card} ${size === "large" ? styles.large : ""}`}
    >
      <AssetImage
        src={creation.cover}
        alt={creation.title}
        className={styles.art}
      >
        <span className={styles.kind}>{KIND_LABEL[creation.kind]}</span>
      </AssetImage>

      <div className={styles.body}>
        <h3 className={styles.title}>{creation.title}</h3>
        <p className={styles.author}>by {creation.author}</p>

        <ul className={styles.metrics}>
          {metrics.map((metric) => renderMetric(metric, creation))}
        </ul>
      </div>
    </Link>
  );
}

function renderMetric(metric: CreationMetric, creation: Creation) {
  switch (metric) {
    case "rating":
      return creation.rating ? (
        <li key={metric} className={`${styles.metric} ${styles.rating}`}>
          <Star {...ICON} />
          {creation.rating.toFixed(1)}
        </li>
      ) : null;

    case "downloads":
      return (
        <li key={metric} className={styles.metric}>
          <Download {...ICON} />
          {formatCount(creation.downloads)}
        </li>
      );

    case "favorites":
      return creation.favorites ? (
        <li key={metric} className={styles.metric}>
          <Heart {...ICON} />
          {formatCount(creation.favorites)}
        </li>
      ) : null;

    case "comments":
      return creation.comments ? (
        <li key={metric} className={styles.metric}>
          <MessageCircle {...ICON} />
          {formatCount(creation.comments)}
        </li>
      ) : null;

    case "frontends":
      return creation.frontends ? (
        <li key={metric} className={`${styles.metric} ${styles.frontends}`}>
          {creation.frontends} frontends
        </li>
      ) : null;
  }
}
