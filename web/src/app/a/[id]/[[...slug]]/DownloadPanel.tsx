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

function costLine(target: DownloadTarget, holdsNothing: boolean): string {
  const lost = target.roles.filter(costs).length;
  if (holdsNothing && lost === 0) return "There is nothing in it yet";
  if (lost === 0) return "Includes everything";
  return lost === 1 ? "1 thing left out" : `${lost} things left out`;
}

function verdictLine(role: RoleVerdict): string {
  if (role.verdict === "dropped") return "Not included.";
  if (role.verdict === "reduced") {
    return role.reason
      ? `Included, without ${role.reason}.`
      : "Included, but not in full.";
  }
  return role.destination ? `Included as ${role.destination}.` : "";
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

type RoleRow = {
  role: string;
  label: string;
  cells: (RoleVerdict | undefined)[];
};

function differingRoles(downloads: DownloadTarget[]): RoleRow[] {
  const order: { role: string; label: string }[] = [];
  for (const target of downloads) {
    for (const role of target.roles) {
      if (!order.some((seen) => seen.role === role.role)) {
        order.push({ role: role.role, label: role.label });
      }
    }
  }

  return order
    .map(({ role, label }) => ({
      role,
      label,
      cells: downloads.map((target) =>
        target.roles.find((entry) => entry.role === role),
      ),
    }))
    .filter((row) => {
      const shapes = row.cells.map((cell) =>
        cell
          ? `${cell.verdict}|${cell.reason ?? ""}|${cell.destination ?? ""}`
          : "absent",
      );
      return new Set(shapes).size > 1;
    });
}

function cellLine(cell: RoleVerdict | undefined): string {
  if (!cell) return "Not included";
  if (cell.verdict === "dropped") return "Not included";
  if (cell.verdict === "reduced") {
    return cell.reason ? `Without ${cell.reason}` : "Not in full";
  }
  return "Included";
}

export function DownloadPanel({
  assetId,
  downloads,
  original,
  images,
  holdsNothing,
  isOwner,
}: {
  assetId: string;
  downloads: DownloadTarget[];
  original: OriginalUpload | null;
  images: AssetImage[];
  /** Whether the asset holds no content, so a format carries a shell of one. */
  holdsNothing: boolean;
  isOwner: boolean;
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
            holdsNothing={holdsNothing}
            primary
          />
          {alternatives.length > 0 ? (
            <FormatChoice
              assetId={assetId}
              downloads={downloads}
              alternatives={alternatives}
            />
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
                    holdsNothing={holdsNothing}
                  />
                </li>
              ))}
            </ul>
          ) : null}
        </>
      )}
      {original && isOwner ? (
        <Original assetId={assetId} original={original} />
      ) : null}
    </section>
  );
}

function FormatChoice({
  assetId,
  downloads,
  alternatives,
}: {
  assetId: string;
  downloads: DownloadTarget[];
  alternatives: DownloadTarget[];
}) {
  const rows = differingRoles(downloads);

  const stake =
    rows.length === 0
      ? "all carry the same content"
      : rows.length === 1
        ? `they differ on ${rows[0].label.toLowerCase()}`
        : `they differ on ${rows.length} things`;

  return (
    <details className={styles.choice}>
      <summary>
        {alternatives.length === 1
          ? "Another format"
          : `${alternatives.length} other formats`}
        <span className={styles.stake}>{stake}</span>
      </summary>
      <ul className={styles.choices}>
        {alternatives.map((target) => (
          <li key={target.format} className={styles.choiceRow}>
            <p className={styles.choiceName}>
              {target.label}
              {target.recommended ? (
                <span className={styles.pick}>Recommended</span>
              ) : null}
            </p>
            {rows.length > 0 ? (
              <ul className={styles.choiceDiff}>
                {rows.map((row) => {
                  const cell = row.cells[downloads.indexOf(target)];
                  const out = !cell || cell.verdict === "dropped";
                  return (
                    <li key={row.role} data-out={out ? "" : undefined}>
                      {row.label}
                      <span>{cellLine(cell)}</span>
                    </li>
                  );
                })}
              </ul>
            ) : null}
            <a
              className={styles.takeAlternative}
              href={`/download/${assetId}/${target.format}`}
            >
              <Download size={15} aria-hidden="true" />
              Download
            </a>
          </li>
        ))}
      </ul>
    </details>
  );
}

function FormatLine({
  assetId,
  target,
  imagesById,
  holdsNothing,
  primary = false,
}: {
  assetId: string;
  target: DownloadTarget;
  imagesById: ReadonlyMap<string, AssetImage>;
  holdsNothing: boolean;
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
      ) : null}
      {primary ? null : (
        <details className={styles.choice}>
          <summary className={styles.label}>{target.label}</summary>
          <div className={styles.choiceBody}>
            {detail.length === 0 ? (
              <p className={styles.cost}>{costLine(target, holdsNothing)}</p>
            ) : (
              <>
                <p className={styles.cost}>{costLine(target, holdsNothing)}</p>
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
              </>
            )}
            <a
              className={styles.takeAlternative}
              href={`/download/${assetId}/${target.format}`}
            >
              <Download size={15} aria-hidden="true" />
              Download {target.label}
            </a>
          </div>
        </details>
      )}
      {!primary ? null : detail.length === 0 ? (
        <p className={styles.cost}>{costLine(target, holdsNothing)}</p>
      ) : (
        <details className={styles.detail}>
          <summary>{costLine(target, holdsNothing)}</summary>
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
