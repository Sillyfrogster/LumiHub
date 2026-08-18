import { Download } from "lucide-react";
import Image from "next/image";
import type { AssetDetail } from "@/lib/api/query";
import styles from "./DownloadPanel.module.css";

type DownloadTarget = AssetDetail["downloads"][number];
type RoleVerdict = DownloadTarget["roles"][number];
type OriginalUpload = NonNullable<AssetDetail["original"]>;
type AssetImage = AssetDetail["media"][number];

/** What a verdict costs. A destination note is a fact, never a loss. */
function costs(role: RoleVerdict): boolean {
  return role.verdict !== "carried";
}

/** The line under a format: either everything travels, or how much does not. */
function costLine(target: DownloadTarget): string {
  const lost = target.roles.filter(costs).length;
  if (lost === 0) return "Everything travels";
  return lost === 1
    ? "1 thing does not travel"
    : `${lost} things do not travel`;
}

/** What one role loses, said plainly enough to decide on. */
function verdictLine(role: RoleVerdict): string {
  if (role.verdict === "dropped") return "Not in this file.";
  if (role.verdict === "reduced") {
    return role.reason ? `Loses ${role.reason}.` : "Does not travel whole.";
  }
  return role.destination ? `Travels as ${role.destination}.` : "";
}

function itemCount(count: number): string {
  return count === 1 ? "1 item" : `${count} items`;
}

/** The file kind in the word a reader would use for it. */
function fileWord(mediaType: string): string {
  if (mediaType.startsWith("image/")) {
    return mediaType.slice("image/".length).toUpperCase();
  }
  if (mediaType === "application/json") return "JSON";
  if (mediaType === "application/zip") return "Archive";
  return "File";
}

function arrivalDate(when: string): string {
  return new Date(when).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

/**
 * The download menu: one line per format Illarin can produce for this asset,
 * each saying what the trip costs before anybody clicks. A format Illarin
 * cannot produce is absent rather than greyed out, so this is a list of
 * choices and not a capability report.
 *
 * The creator sees this component, with these words. There is no
 * creator-flavoured paraphrase of the same facts.
 */
export function DownloadPanel({
  assetId,
  downloads,
  original,
  images,
}: {
  assetId: string;
  downloads: DownloadTarget[];
  original: OriginalUpload | null;
  images: AssetImage[];
}) {
  if (downloads.length === 0 && !original) return null;

  return (
    <section className={styles.panel} aria-labelledby="downloads-heading">
      <h2 id="downloads-heading">Download</h2>
      {downloads.length > 0 ? (
        <ul className={styles.formats}>
          {downloads.map((target) => (
            <li key={target.format}>
              <FormatLine assetId={assetId} target={target} images={images} />
            </li>
          ))}
        </ul>
      ) : (
        <p className={styles.none}>
          Illarin cannot write this one out yet. The creator’s own file is
          below.
        </p>
      )}
      {original ? <Original assetId={assetId} original={original} /> : null}
    </section>
  );
}

function FormatLine({
  assetId,
  target,
  images,
}: {
  assetId: string;
  target: DownloadTarget;
  images: AssetImage[];
}) {
  const lost = target.roles.filter(costs);
  const notes = target.roles.filter((role) => !costs(role) && role.destination);
  const detail = [...lost, ...notes];

  return (
    <div className={styles.format}>
      <p className={styles.label}>
        {target.label}
        {target.recommended ? (
          <span className={styles.recommended}>Recommended</span>
        ) : null}
      </p>
      {detail.length === 0 ? (
        <p className={styles.cost}>{costLine(target)}</p>
      ) : (
        <details className={styles.detail}>
          <summary>{costLine(target)}</summary>
          <ul className={styles.losses}>
            {detail.map((role) => (
              <li key={role.role}>
                <p className={styles.lossName}>
                  {role.label}
                  <span className={styles.lossCount}>
                    — {itemCount(role.sample.count)}
                  </span>
                </p>
                <p className={styles.lossWhy}>{verdictLine(role)}</p>
                <Sample role={role} images={images} />
              </li>
            ))}
          </ul>
        </details>
      )}
      <a className={styles.take} href={`/download/${assetId}/${target.format}`}>
        <Download size={15} aria-hidden="true" />
        Download {target.label}
      </a>
    </div>
  );
}

/**
 * A glance at what is at stake — entry names, greeting openings, a strip of
 * the pictures — so somebody recognises in two seconds whether they care.
 */
function Sample({ role, images }: { role: RoleVerdict; images: AssetImage[] }) {
  const pictures = (role.sample.images ?? [])
    .map((id) => images.find((image) => image.id === id))
    .filter((image): image is AssetImage => image !== undefined);
  if (pictures.length > 0) {
    return (
      <ul className={styles.strip}>
        {pictures.map((picture) => (
          <li key={picture.id}>
            <Image
              src={picture.thumbUrl}
              alt=""
              width={40}
              height={40}
              unoptimized
            />
          </li>
        ))}
      </ul>
    );
  }
  const texts = role.sample.texts ?? [];
  if (texts.length === 0) return null;
  return (
    <ul className={styles.examples}>
      {texts.map((text, index) => (
        // Two greetings may open with the same words, so position is the key.
        <li key={`${index}-${text}`}>{text}</li>
      ))}
    </ul>
  );
}

/**
 * The creator's own upload, on its own below the generated downloads. It is
 * never in the list beside them and never recommended, because a year-old file
 * is not the current work.
 */
function Original({
  assetId,
  original,
}: {
  assetId: string;
  original: OriginalUpload;
}) {
  return (
    <div className={styles.original}>
      <h3>The creator’s own file</h3>
      <p>
        {original.label ? `${original.label} · ` : ""}
        {fileWord(original.mediaType)}, uploaded{" "}
        {arrivalDate(original.arrivedAt)}.
      </p>
      <p className={styles.sinceThen}>Edits made since are not in this file.</p>
      <a className={styles.takeOriginal} href={`/download/${assetId}`}>
        <Download size={15} aria-hidden="true" />
        Download the original
      </a>
    </div>
  );
}
