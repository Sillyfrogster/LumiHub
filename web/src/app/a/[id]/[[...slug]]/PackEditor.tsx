"use client";

import { ImagePlus, UserRound, X } from "lucide-react";
import Image from "next/image";
import { useMemo, useRef, useState } from "react";
import {
  type AssetImage,
  addAssetImage,
  type LumiaRecord,
  type RecordListContent,
} from "@/lib/api/query";
import {
  CollectionEditor,
  Field,
  FieldGroup,
  FieldPair,
  ItemFields,
  ItemHeading,
  NothingChosen,
  replaceAt,
  without,
} from "./CollectionEditor";
import styles from "./PackEditor.module.css";

const PRONOUNS: Array<{
  value: LumiaRecord["genderIdentity"];
  label: string;
}> = [
  { value: 0, label: "She / her" },
  { value: 1, label: "He / him" },
  { value: 2, label: "They / them" },
];

function recordName(record: LumiaRecord, position: number): string {
  return record.lumiaName.trim() || `Lumia ${position + 1}`;
}

export function PackEditor({
  assetId,
  content,
  images,
  pending,
  onChange,
  onImageAdded,
}: {
  assetId: string;
  content: RecordListContent;
  images: AssetImage[];
  pending: boolean;
  onChange: (content: RecordListContent) => void;
  onImageAdded: () => void;
}) {
  const [selected, setSelected] = useState(0);
  const records = content.records;
  const current = records[selected];

  function replaceCurrent(changes: Partial<LumiaRecord>) {
    onChange({ ...content, records: replaceAt(records, selected, changes) });
  }

  return (
    <CollectionEditor
      noun="Lumia"
      emptyMessage="This pack has no Lumia yet."
      pending={pending}
      selected={selected}
      onSelect={setSelected}
      onAdd={() =>
        onChange({
          ...content,
          records: [
            ...records,
            {
              lumiaName: "",
              lumiaDefinition: "",
              lumiaPersonality: "",
              lumiaBehavior: "",
              genderIdentity: 2,
              authorName: "",
              version: 1,
            },
          ],
        })
      }
      rows={records.map((record, index) => ({
        name: recordName(record, index),
        detail: record.authorName.trim() || "No author named",
        search: [record.lumiaName, record.authorName, record.lumiaDefinition]
          .join(" ")
          .toLowerCase(),
      }))}
    >
      {current ? (
        <LumiaFields
          assetId={assetId}
          record={current}
          position={selected}
          images={images}
          pending={pending}
          onChange={replaceCurrent}
          onImageAdded={onImageAdded}
          onRemove={() => {
            onChange({ ...content, records: without(records, selected) });
            setSelected(Math.max(0, Math.min(selected, records.length - 2)));
          }}
        />
      ) : (
        <NothingChosen>
          Choose a Lumia to edit it, or add the first one.
        </NothingChosen>
      )}
    </CollectionEditor>
  );
}

function LumiaFields({
  assetId,
  record,
  position,
  images,
  pending,
  onChange,
  onImageAdded,
  onRemove,
}: {
  assetId: string;
  record: LumiaRecord;
  position: number;
  images: AssetImage[];
  pending: boolean;
  onChange: (changes: Partial<LumiaRecord>) => void;
  onImageAdded: () => void;
  onRemove: () => void;
}) {
  return (
    <ItemFields>
      <ItemHeading
        name={recordName(record, position)}
        noun="Lumia"
        pending={pending}
        onRemove={onRemove}
      />

      <AvatarField
        assetId={assetId}
        record={record}
        images={images}
        pending={pending}
        onChange={onChange}
        onImageAdded={onImageAdded}
      />

      <Field label="Name">
        <input
          value={record.lumiaName}
          onChange={(event) => onChange({ lumiaName: event.target.value })}
          disabled={pending}
        />
      </Field>

      <FieldGroup legend="Credit and identity">
        <FieldPair>
          <Field label="Author">
            <input
              value={record.authorName}
              onChange={(event) => onChange({ authorName: event.target.value })}
              disabled={pending}
            />
          </Field>
          <Field label="Version">
            <input
              type="number"
              min={1}
              step={1}
              value={record.version}
              onChange={(event) =>
                onChange({ version: Math.max(1, Number(event.target.value)) })
              }
              disabled={pending}
            />
          </Field>
        </FieldPair>
        <Field label="Pronouns">
          <select
            value={record.genderIdentity}
            onChange={(event) =>
              onChange({
                genderIdentity: Number(
                  event.target.value,
                ) as LumiaRecord["genderIdentity"],
              })
            }
            disabled={pending}
          >
            {PRONOUNS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </Field>
      </FieldGroup>

      <Field label="Definition">
        <textarea
          rows={6}
          value={record.lumiaDefinition}
          onChange={(event) =>
            onChange({ lumiaDefinition: event.target.value })
          }
          disabled={pending}
        />
      </Field>
      <Field label="Personality">
        <textarea
          rows={6}
          value={record.lumiaPersonality}
          onChange={(event) =>
            onChange({ lumiaPersonality: event.target.value })
          }
          disabled={pending}
        />
      </Field>
      <Field label="Behaviour">
        <textarea
          rows={6}
          value={record.lumiaBehavior}
          onChange={(event) => onChange({ lumiaBehavior: event.target.value })}
          disabled={pending}
        />
      </Field>
    </ItemFields>
  );
}

function AvatarField({
  assetId,
  record,
  images,
  pending,
  onChange,
  onImageAdded,
}: {
  assetId: string;
  record: LumiaRecord;
  images: AssetImage[];
  pending: boolean;
  onChange: (changes: Partial<LumiaRecord>) => void;
  onImageAdded: () => void;
}) {
  const [uploading, setUploading] = useState(false);
  const [message, setMessage] = useState("");
  const [preview, setPreview] = useState("");
  const file = useRef<HTMLInputElement>(null);
  const imagesById = useMemo(
    () => new Map(images.map((image) => [image.id, image])),
    [images],
  );
  const stored = record.avatarUrl
    ? imagesById.get(record.avatarUrl)
    : undefined;
  const source = preview || stored?.thumbUrl;

  async function upload(chosen: File | null) {
    if (!chosen || uploading) return;
    setUploading(true);
    setMessage("");
    try {
      const mediaId = await addAssetImage(assetId, chosen, "pack_item");
      setPreview(URL.createObjectURL(chosen));
      onChange({ avatarUrl: mediaId });
      onImageAdded();
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "The avatar could not be added. Try again.",
      );
    } finally {
      setUploading(false);
      if (file.current) file.current.value = "";
    }
  }

  return (
    <div className={styles.avatarField}>
      <div className={styles.avatarPreview}>
        {source ? (
          <Image
            src={source}
            alt=""
            width={stored?.width ?? 240}
            height={stored?.height ?? 240}
            sizes="120px"
            unoptimized
          />
        ) : (
          <UserRound size={34} strokeWidth={1.35} aria-hidden="true" />
        )}
      </div>
      <div className={styles.avatarActions}>
        <p>Avatar</p>
        <span>Square images work best. The source Pack is left untouched.</span>
        {message ? (
          <span className={styles.error} role="alert">
            {message}
          </span>
        ) : null}
        <div>
          <label className={styles.upload}>
            <ImagePlus size={16} aria-hidden="true" />
            {uploading ? "Adding…" : source ? "Replace avatar" : "Add avatar"}
            <input
              ref={file}
              type="file"
              accept="image/*"
              disabled={pending || uploading}
              onChange={(event) => void upload(event.target.files?.[0] ?? null)}
            />
          </label>
          {record.avatarUrl ? (
            <button
              type="button"
              className={styles.removeAvatar}
              disabled={pending || uploading}
              onClick={() => {
                setPreview("");
                onChange({ avatarUrl: undefined });
              }}
            >
              <X size={15} aria-hidden="true" />
              Remove avatar
            </button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
