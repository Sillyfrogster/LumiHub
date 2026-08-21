import { Download } from "lucide-react";
import Image from "next/image";
import type { AssetDetail } from "@/lib/api/query";
import styles from "./DownloadPanel.module.css";

type DownloadTarget = AssetDetail["downloads"][number];
type RoleVerdict = DownloadTarget["roles"][number];
type OriginalUpload = NonNullable<AssetDetail["original"]>;
type AssetImage = AssetDetail["media"][number];

function costs(role: RoleVerdict): boolean {
  return role.verdict !== "carried";
}

function costLine(target: DownloadTarget): string {
  const lost = target.roles.filter(costs).length;
  if (lost === 0) return "Everything travels";
  return lost === 1
    ? "1 thing does not travel"
    : `${lost} things do not travel`;
}

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
  const imagesById = new Map(images.map((image) => [image.id, image]));
  const recommended = downloads.find((target) => target.recommended);
  const alternatives = recommended
    ? downloads.filter((target) => target.format !== recommended.format)
    : downloads;

  return (
    <section className={styles.panel} aria-labelledby="downloads-heading">
      <h2 id="downloads-heading">Download</h2>
      {recommended ? (
        <>
          <FormatLine
            assetId={assetId}
            target={recommended}
            imagesById={imagesById}
            primary
          />
          {alternatives.length > 0 ? (
            <details className={styles.otherFormats}>
              <summary>
                {alternatives.length === 1
                  ? "Another format"
                  : `${alternatives.length} other formats`}
              </summary>
              <ul className={styles.formats}>
                {alternatives.map((target) => (
                  <li key={target.format}>
                    <FormatLine
                      assetId={assetId}
                      target={target}
                      imagesById={imagesById}
                    />
                  </li>
                ))}
              </ul>
            </details>
          ) : null}
        </>
      ) : (
        <>
          <p className={styles.none}>
            {downloads.length > 0
              ? "No recommended format is available."
              : "Illarin cannot write this one out yet. The creator’s own file is below."}
          </p>
          {alternatives.length > 0 ? (
            <ul className={styles.formats}>
              {alternatives.map((target) => (
                <li key={target.format}>
                  <FormatLine
                    assetId={assetId}
                    target={target}
                    imagesById={imagesById}
                  />
                </li>
              ))}
            </ul>
          ) : null}
        </>
      )}
      {original ? <Original assetId={assetId} original={original} /> : null}
    </section>
  );
}

function FormatLine({
  assetId,
  target,
  imagesById,
  primary = false,
}: {
  assetId: string;
  target: DownloadTarget;
  imagesById: ReadonlyMap<string, AssetImage>;
  primary?: boolean;
}) {
  const lost = target.roles.filter(costs);
  const notes = target.roles.filter((role) => !costs(role) && role.destination);
  const detail = [...lost, ...notes];

  return (
    <div className={primary ? styles.primaryFormat : styles.format}>
      {primary ? (
        <p className={styles.recommended}>Recommended format</p>
      ) : null}
      {primary ? (
        <a
          className={styles.take}
          href={`/download/${assetId}/${target.format}`}
        >
          <Download size={15} aria-hidden="true" />
          Download {target.label}
        </a>
      ) : (
        <p className={styles.label}>{target.label}</p>
      )}
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
                <Sample role={role} imagesById={imagesById} />
              </li>
            ))}
          </ul>
        </details>
      )}
      {primary ? null : (
        <a
          className={styles.takeAlternative}
          href={`/download/${assetId}/${target.format}`}
        >
          <Download size={15} aria-hidden="true" />
          Download {target.label}
        </a>
      )}
    </div>
  );
}

function Sample({
  role,
  imagesById,
}: {
  role: RoleVerdict;
  imagesById: ReadonlyMap<string, AssetImage>;
}) {
  const pictures = (role.sample.images ?? [])
    .map((id) => imagesById.get(id))
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
