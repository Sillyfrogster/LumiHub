"use client";

import { Maximize2 } from "lucide-react";
import Image from "next/image";
import {
  type CSSProperties,
  Fragment,
  useEffect,
  useRef,
  useState,
} from "react";
import { ChipSet } from "@/components/ui/Chip";
import { FormattingNotice, RichText } from "@/components/ui/RichText";
import type {
  AssetElement,
  AssetImage,
  PresetSetting,
  PresetVariable,
  PromptListContent,
  RegexScript,
  TypedValue,
} from "@/lib/api/query";
import { elementLabel } from "@/lib/element-label";
import {
  contentItemCount,
  excerptDefinition,
  opensFullScreen,
} from "@/lib/page-arrangement";
import { type NamedSlot, nameSlot, orderSettings } from "@/lib/preset-slots";
import { formattingWasRemoved, richTextsOf } from "@/lib/rich-text";
import styles from "./ElementBody.module.css";
import { Lorebook } from "./Lorebook";

const ITEM_WIDTHS = { small: "120px", medium: "180px", large: "260px" };

const KEY_PREVIEW_LIMIT = 6;

export function ElementBody({
  element,
  isOwner,
  images = [],
  blockTitle,
  blockElements = 2,
  onExpand,
  onReadMore,
}: {
  element: AssetElement;
  isOwner: boolean;
  images?: AssetImage[];
  blockTitle?: string;
  /** How many elements the block renders, this one included. */
  blockElements?: number;
  onExpand?: () => void;
  onReadMore?: () => void;
}) {
  if (element.isEmpty && !isOwner) return null;

  const label = elementLabel(element, {
    title: blockTitle,
    elements: blockElements,
  });
  const expandable = isOwner && onExpand && opensFullScreen(element.type);

  return (
    <section className={styles.element}>
      {label || expandable ? (
        <div className={styles.heading}>
          {label ? <h3 className={styles.label}>{label}</h3> : <span />}
          {expandable ? (
            <button
              type="button"
              className={styles.expand}
              data-measurement-ignore
              onClick={onExpand}
              aria-label={`Edit ${element.label || "this content"} in full screen`}
            >
              <Maximize2 size={14} aria-hidden="true" />
              Edit in full screen
            </button>
          ) : null}
        </div>
      ) : null}
      {element.isEmpty ? (
        <p className={styles.blank}>Empty</p>
      ) : (
        <>
          <ExcerptedElementContent
            element={element}
            images={images}
            onReadMore={onReadMore}
          />
          {formattingWasRemoved(richTextsOf(element)) ? (
            <FormattingNotice />
          ) : null}
        </>
      )}
    </section>
  );
}

function ExcerptedElementContent({
  element,
  images,
  onReadMore,
}: {
  element: AssetElement;
  images: AssetImage[];
  onReadMore?: () => void;
}) {
  const definition = excerptDefinition(element.type);
  const excerpt = useRef<HTMLDivElement>(null);
  const [lineCut, setLineCut] = useState(false);
  const itemCount = visibleItemCount(element);
  const hasItemCut =
    definition.unit === "items" && itemCount > definition.limit;

  useEffect(() => {
    if (definition.unit !== "lines") return;

    const node = excerpt.current;
    if (!node) return;
    const measure = () => {
      setLineCut(node.scrollHeight - node.clientHeight > 1);
    };
    const observer = new ResizeObserver(measure);
    measure();
    observer.observe(node);
    return () => observer.disconnect();
  }, [definition.unit]);

  const isCut = definition.unit === "lines" ? lineCut : hasItemCut;
  const itemLimit = definition.unit === "items" ? definition.limit : undefined;

  if (definition.unit === "self") {
    return <ElementContent element={element} images={images} />;
  }

  return (
    <>
      <div
        ref={excerpt}
        data-line-excerpt={definition.unit === "lines" ? true : undefined}
        className={
          definition.unit === "lines" ? styles.lineExcerpt : styles.itemExcerpt
        }
        data-image-excerpt={
          element.type === "image_set" && isCut ? true : undefined
        }
        data-truncated={isCut ? true : undefined}
        style={
          definition.unit === "lines"
            ? ({ "--excerpt-lines": definition.limit } as CSSProperties)
            : undefined
        }
      >
        <ElementContent
          element={element}
          images={images}
          itemLimit={itemLimit}
        />
      </div>
      {isCut && onReadMore ? (
        <button
          id={`read-${element.id}`}
          type="button"
          className={styles.readMore}
          data-read-more
          onClick={onReadMore}
        >
          {excerptControlLabel(element, itemCount)}
        </button>
      ) : null}
    </>
  );
}

function visibleItemCount(element: AssetElement): number {
  if (element.type === "setting_group" && "settings" in element.content) {
    return element.content.settings.filter((setting) => setting.value != null)
      .length;
  }
  return contentItemCount(element);
}

function excerptControlLabel(element: AssetElement, itemCount: number): string {
  if (element.type === "prose") {
    return `the rest of ${element.label.trim().toLocaleLowerCase() || "this text"}`;
  }
  return `all ${itemCount} ${excerptNoun(element)}`;
}

function excerptNoun(element: AssetElement): string {
  switch (element.type) {
    case "text_set":
      return element.role === "prompt_nudges" ? "nudges" : "items";
    case "field_list":
      return element.label.toLocaleLowerCase().includes("attribute")
        ? "attributes"
        : "fields";
    case "dialogue_sample":
      return "turns";
    case "image_set":
      return element.role === "expressions" ? "expressions" : "images";
    case "link_list":
      return "links";
    case "prompt_list":
      return "fragments";
    case "variable_schema":
      return "variables";
    case "setting_group":
      return "settings";
    case "script_list":
      return "scripts";
    default:
      return "items";
  }
}

export function ElementContent({
  element,
  images,
  itemLimit,
}: {
  element: AssetElement;
  images: AssetImage[];
  itemLimit?: number;
}) {
  const { content } = element;

  if (element.type === "prose" && "text" in content) {
    return element.display === "verbatim" ? (
      <pre className={`${styles.verbatim} ${styles.proseMeasure}`}>
        {content.text}
      </pre>
    ) : (
      <RichText text={content.text} className={styles.proseMeasure} />
    );
  }

  if (element.type === "text_set" && "texts" in content) {
    const verbatim = element.display === "verbatim";
    const named = element.role === "prompt_nudges";
    return (
      <ol className={styles.textSet}>
        {content.texts.slice(0, itemLimit).map((item, index) => (
          <li key={`${index}-${item.name ?? ""}`}>
            {item.name ? (
              <p className={styles.itemName}>
                {named ? nameSlot(item.name).name : item.name}
              </p>
            ) : null}
            {verbatim ? (
              <pre className={styles.verbatim}>{item.text}</pre>
            ) : (
              <RichText text={item.text} />
            )}
          </li>
        ))}
      </ol>
    );
  }

  if (element.type === "dialogue_sample" && "turns" in content) {
    return (
      <ol className={styles.dialogue}>
        {content.turns.slice(0, itemLimit).map((turn, index) => (
          <li key={`${index}-${turn.speaker}`}>
            <p className={styles.speaker}>{turn.speaker}</p>
            <RichText text={turn.text} />
          </li>
        ))}
      </ol>
    );
  }

  if (element.type === "field_list" && "fields" in content) {
    return (
      <dl className={styles.fieldList}>
        {content.fields.slice(0, itemLimit).map((field, index) => (
          <Fragment key={`${index}-${field.name ?? ""}`}>
            <dt>{field.name || "Unnamed"}</dt>
            <dd>
              <RichText text={field.value} className={styles.tight} />
            </dd>
          </Fragment>
        ))}
      </dl>
    );
  }

  if (element.type === "link_list" && "links" in content) {
    return (
      <ul className={styles.linkList}>
        {content.links.slice(0, itemLimit).map((link, index) => (
          <li key={`${index}-${link.url}`}>
            <a href={link.url} rel="noreferrer nofollow" target="_blank">
              {link.label || link.url}
            </a>
            {link.note ? (
              <RichText text={link.note} className={styles.note} />
            ) : null}
          </li>
        ))}
      </ul>
    );
  }

  if (element.type === "image_set" && "images" in content) {
    const width = ITEM_WIDTHS[element.itemSize ?? "medium"];
    const imagesById = new Map(images.map((image) => [image.id, image]));
    return (
      <ul
        className={styles.imageSet}
        style={{ "--item-width": width } as CSSProperties}
      >
        {content.images.slice(0, itemLimit).map((item) => {
          const image = imagesById.get(item.mediaId);
          if (!image) return null;
          return (
            <li key={item.mediaId}>
              <Image
                src={image.thumbUrl}
                alt={item.name || ""}
                width={image.width}
                height={image.height}
                sizes="260px"
                unoptimized
              />
              {item.name ? <span>{item.name}</span> : null}
            </li>
          );
        })}
      </ul>
    );
  }

  if (element.type === "entry_table" && "entries" in content) {
    return <Lorebook entries={content.entries} />;
  }

  if (element.type === "prompt_list" && "fragments" in content) {
    return <PromptList content={content} itemLimit={itemLimit} />;
  }

  if (element.type === "setting_group" && "settings" in content) {
    return <SettingGroup settings={content.settings} itemLimit={itemLimit} />;
  }

  if (element.type === "variable_schema" && "variables" in content) {
    return (
      <VariableSchema variables={content.variables} itemLimit={itemLimit} />
    );
  }

  if (element.type === "script_list" && "scripts" in content) {
    return <ScriptList scripts={content.scripts} itemLimit={itemLimit} />;
  }

  return null;
}

const PROMPT_ROLE_LABELS: Record<string, string> = {
  system: "System",
  user: "User",
  assistant: "Assistant",
  user_append: "User, appended",
  assistant_append: "Assistant, appended",
};

const PLACEMENT_LABELS: Record<string, string> = {
  pre_history: "Before the conversation",
  in_history: "In the conversation",
  post_history: "After the conversation",
};

function PromptList({
  content,
  itemLimit,
}: {
  content: PromptListContent;
  itemLimit?: number;
}) {
  const groups = content.groups ?? [];
  const fragments = (content.fragments ?? []).slice(0, itemLimit);
  const groupNames = new Map(groups.map((group) => [group.id, group.name]));
  const runs: { group?: string; fragments: PromptListContent["fragments"] }[] =
    [];
  for (const fragment of fragments) {
    const name = fragment.groupId
      ? groupNames.get(fragment.groupId)
      : undefined;
    const open = runs.at(-1);
    if (open && open.group === name) open.fragments.push(fragment);
    else runs.push({ group: name, fragments: [fragment] });
  }
  return (
    <div className={styles.promptList}>
      {runs.map((run, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Runs hold no local state.
        <section className={styles.promptGroup} key={index}>
          {run.group ? <h4>{run.group}</h4> : null}
          <Fragments fragments={run.fragments} />
        </section>
      ))}
    </div>
  );
}

function Fragments({
  fragments,
}: {
  fragments: PromptListContent["fragments"];
}) {
  return (
    <ol className={styles.fragments}>
      {fragments.map((fragment, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: Fragments hold no local state.
        <li key={index} data-off={fragment.enabled ? undefined : true}>
          <div className={styles.fragmentHead}>
            <span className={styles.fragmentName}>
              {fragment.name?.trim() ||
                fragment.marker?.trim() ||
                `Fragment ${index + 1}`}
            </span>
            <span className={styles.fragmentTags}>
              {fragment.role ? PROMPT_ROLE_LABELS[fragment.role] : null}
              {fragment.placement
                ? ` · ${PLACEMENT_LABELS[fragment.placement]}`
                : null}
              {fragment.enabled ? null : " · Off"}
            </span>
          </div>
          {fragment.marker ? (
            <p className={styles.fragmentMarker}>
              The app splices its own content in here.
            </p>
          ) : (
            <Paragraphs text={fragment.text} />
          )}
        </li>
      ))}
    </ol>
  );
}

function SettingGroup({
  settings,
  itemLimit,
}: {
  settings: PresetSetting[];
  itemLimit?: number;
}) {
  const shown = orderSettings(
    settings.filter((setting) => setting.value != null),
  ).slice(0, itemLimit);
  if (shown.length === 0) return null;
  const named = shown.filter((setting) => setting.slot.rank !== "unrecognised");
  const raw = shown.filter((setting) => setting.slot.rank === "unrecognised");
  return (
    <>
      {named.length > 0 ? <Settings settings={named} /> : null}
      {raw.length > 0 ? (
        <>
          <p className={styles.rawSettingsCaption}>As the file names them</p>
          <Settings settings={raw} raw />
        </>
      ) : null}
    </>
  );
}

function Settings({
  settings,
  raw,
}: {
  settings: Array<PresetSetting & { slot: NamedSlot }>;
  raw?: boolean;
}) {
  return (
    <dl className={styles.settings} data-raw={raw ? true : undefined}>
      {settings.map((setting) => (
        <div className={styles.setting} key={setting.id ?? setting.name}>
          <dt>{setting.slot.name}</dt>
          <dd>
            <SettingValue value={setting.value} />
            {setting.slot.note ? (
              <span className={styles.settingNote}>{setting.slot.note}</span>
            ) : null}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * A setting's value. Text made only of spaces and newlines is shown as it is
 * written, because saying a setting holds nothing when it holds two blank
 * lines is a lie a reader would act on.
 */
function SettingValue({ value }: { value: TypedValue | undefined }) {
  if (value?.text != null && value.text !== "" && value.text.trim() === "") {
    return <code className={styles.whitespaceValue}>{value.text}</code>;
  }
  return <>{writeValue(value)}</>;
}

function writeValue(value: TypedValue | undefined): string {
  if (!value) return "";
  if (value.number != null) {
    return value.number.toLocaleString("en-GB", { maximumFractionDigits: 20 });
  }
  if (value.boolean != null) return value.boolean ? "Yes" : "No";
  if (value.strings) {
    return value.strings.length === 0 ? "Nothing" : value.strings.join(", ");
  }
  return value.text === "" ? "Nothing" : (value.text ?? "");
}

function VariableSchema({
  variables,
  itemLimit,
}: {
  variables: PresetVariable[];
  itemLimit?: number;
}) {
  return (
    <ul className={styles.variables}>
      {variables.slice(0, itemLimit).map((variable, index) => (
        <li key={variable.id ?? `${index}-${variable.name}`}>
          <p className={styles.itemName}>
            {variable.label?.trim() || variable.name}
          </p>
          {variable.description ? (
            <RichText text={variable.description} />
          ) : null}
          {variable.options && variable.options.length > 0 ? (
            <ChipSet
              className={styles.choices}
              limit={KEY_PREVIEW_LIMIT}
              items={variable.options.map((option, position) => ({
                id: `${position}-${option.value}`,
                label: option.label || option.value,
              }))}
            />
          ) : null}
        </li>
      ))}
    </ul>
  );
}

function ScriptList({
  scripts,
  itemLimit,
}: {
  scripts: RegexScript[];
  itemLimit?: number;
}) {
  return (
    <ul className={styles.scripts}>
      {scripts.slice(0, itemLimit).map((script, index) => (
        <li
          key={script.id ?? index}
          data-off={script.enabled ? undefined : true}
        >
          <p className={styles.itemName}>
            {script.name?.trim() || `Script ${index + 1}`}
            {script.enabled ? null : (
              <span className={styles.fragmentTags}> · Off</span>
            )}
          </p>
          <p className={styles.pattern}>
            <code>{script.find}</code>
            <span aria-hidden="true">→</span>
            <code>{script.replace || "nothing"}</code>
          </p>
        </li>
      ))}
    </ul>
  );
}

function Paragraphs({ text }: { text: string }) {
  const paragraphs = text.split(/\n{2,}/).filter((line) => line.trim() !== "");
  return (
    <>
      {paragraphs.map((paragraph, index) => (
        <p key={`${index}-${paragraph.slice(0, 24)}`}>{paragraph}</p>
      ))}
    </>
  );
}
