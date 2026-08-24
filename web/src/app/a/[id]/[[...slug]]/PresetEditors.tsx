"use client";

import { Plus, Trash2, X } from "lucide-react";
import { useState } from "react";
import type {
  PresetSetting,
  PresetVariable,
  PromptFragment,
  PromptGroup,
  PromptListContent,
  RegexScript,
  TypedValue,
} from "@/lib/api/query";
import { nameSlot } from "@/lib/preset-slots";
import {
  CollectionEditor,
  CollectionStack,
  Field,
  FieldGroup,
  FieldPair,
  ItemFields,
  ItemHeading,
  NothingChosen,
  readLines,
  replaceAt,
  Switch,
  without,
  writeLines,
} from "./CollectionEditor";
import styles from "./PresetEditors.module.css";

const PROMPT_ROLES: {
  value: NonNullable<PromptFragment["role"]>;
  label: string;
}[] = [
  { value: "system", label: "System" },
  { value: "user", label: "User" },
  { value: "assistant", label: "Assistant" },
  { value: "user_append", label: "User, added to the message before it" },
  {
    value: "assistant_append",
    label: "Assistant, added to the message before it",
  },
];

const PLACEMENTS: {
  value: NonNullable<PromptFragment["placement"]>;
  label: string;
}[] = [
  { value: "pre_history", label: "Before the conversation" },
  { value: "in_history", label: "Inside the conversation" },
  { value: "post_history", label: "After the conversation" },
];

const WIDGETS: { value: PresetVariable["widget"]; label: string }[] = [
  { value: "switch", label: "Yes or no" },
  { value: "select", label: "One of a list" },
  { value: "multiselect", label: "Several of a list" },
  { value: "number", label: "A number" },
  { value: "slider", label: "A number on a slider" },
  { value: "text", label: "A line of text" },
  { value: "textarea", label: "Several lines of text" },
];

const SCRIPT_TARGETS: {
  value: NonNullable<RegexScript["targets"]>[number];
  label: string;
}[] = [
  { value: "user_input", label: "What the reader writes" },
  { value: "model_output", label: "What the model writes back" },
  { value: "slash_command", label: "What a command produces" },
  { value: "lorebook", label: "What the lorebook adds" },
];

const SCRIPT_EFFECTS: {
  value: NonNullable<RegexScript["affects"]>[number];
  label: string;
}[] = [
  { value: "display", label: "What a person is shown" },
  { value: "prompt", label: "What the model is sent" },
];

export function fragmentName(
  fragment: PromptFragment,
  position: number,
): string {
  if (fragment.name?.trim()) return fragment.name;
  if (fragment.marker?.trim()) return fragment.marker;
  return `Fragment ${position + 1}`;
}

export function PromptListEditor({
  content,
  pending,
  onChange,
}: {
  content: PromptListContent;
  pending: boolean;
  onChange: (content: PromptListContent) => void;
}) {
  const [selected, setSelected] = useState(0);
  const groups = content.groups ?? [];
  const fragments = content.fragments ?? [];
  const groupNames = new Map(groups.map((group) => [group.id, group.name]));
  const current = fragments[selected];

  function replaceCurrent(changes: Partial<PromptFragment>) {
    onChange({ groups, fragments: replaceAt(fragments, selected, changes) });
  }

  return (
    <CollectionStack
      above={
        <GroupEditor
          groups={groups}
          fragments={fragments}
          pending={pending}
          onChange={onChange}
        />
      }
    >
      <CollectionEditor
        noun="fragment"
        emptyMessage="This preset has no prompt fragments yet."
        pending={pending}
        selected={selected}
        onSelect={setSelected}
        onAdd={() =>
          onChange({
            groups,
            fragments: [
              ...fragments,
              { role: "system", text: "", enabled: true },
            ],
          })
        }
        rows={fragments.map((fragment, index) => ({
          name: fragmentName(fragment, index),
          detail: fragment.groupId
            ? groupNames.get(fragment.groupId)
            : undefined,
          off: !fragment.enabled,
          sealed: fragment.protected ?? false,
          search: [
            fragmentName(fragment, index),
            fragment.text,
            fragment.marker ?? "",
          ]
            .join(" ")
            .toLowerCase(),
        }))}
      >
        {current ? (
          <FragmentFields
            fragment={current}
            position={selected}
            groups={groups}
            pending={pending}
            onChange={replaceCurrent}
            onRemove={() => {
              onChange({ groups, fragments: without(fragments, selected) });
              setSelected(
                Math.max(0, Math.min(selected, fragments.length - 2)),
              );
            }}
          />
        ) : (
          <NothingChosen>
            Choose a fragment to edit it, or add the first one.
          </NothingChosen>
        )}
      </CollectionEditor>
    </CollectionStack>
  );
}

function GroupEditor({
  groups,
  fragments,
  pending,
  onChange,
}: {
  groups: PromptGroup[];
  fragments: PromptFragment[];
  pending: boolean;
  onChange: (content: PromptListContent) => void;
}) {
  const [adding, setAdding] = useState("");

  // Fragments can reference a new heading before the next save.
  function addGroup() {
    if (adding.trim() === "") return;
    onChange({
      groups: [...groups, { id: crypto.randomUUID(), name: adding.trim() }],
      fragments,
    });
    setAdding("");
  }

  function removeGroup(index: number) {
    const gone = groups[index].id;
    onChange({
      groups: without(groups, index),
      fragments: fragments.map((fragment) =>
        fragment.groupId === gone
          ? { ...fragment, groupId: undefined }
          : fragment,
      ),
    });
  }

  return (
    <FieldGroup legend="Headings">
      {groups.length === 0 ? (
        <p className={styles.quiet}>
          Fragments run in one list until you add a heading to group them under.
        </p>
      ) : (
        <ul className={styles.groups}>
          {groups.map((group, index) => (
            <li key={group.id ?? index} className={styles.group}>
              <input
                aria-label={`Heading ${index + 1}`}
                size={Math.max(8, Math.min(28, group.name.length + 1))}
                value={group.name}
                onChange={(event) =>
                  onChange({
                    groups: replaceAt(groups, index, {
                      name: event.target.value,
                    }),
                    fragments,
                  })
                }
                disabled={pending}
              />
              <button
                type="button"
                className={styles.removeGroup}
                aria-label={`Remove the heading ${group.name}`}
                onClick={() => removeGroup(index)}
                disabled={pending}
              >
                <X size={14} aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className={styles.addGroup}>
        <input
          aria-label="A new heading"
          placeholder="A new heading"
          value={adding}
          onChange={(event) => setAdding(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter") return;
            event.preventDefault();
            addGroup();
          }}
          disabled={pending}
        />
        <button
          type="button"
          onClick={addGroup}
          disabled={pending || adding.trim() === ""}
        >
          <Plus size={16} aria-hidden="true" />
          Add heading
        </button>
      </div>
    </FieldGroup>
  );
}

function FragmentFields({
  fragment,
  position,
  groups,
  pending,
  onChange,
  onRemove,
}: {
  fragment: PromptFragment;
  position: number;
  groups: PromptGroup[];
  pending: boolean;
  onChange: (changes: Partial<PromptFragment>) => void;
  onRemove: () => void;
}) {
  const isMarker = (fragment.marker ?? "") !== "";
  return (
    <ItemFields>
      <ItemHeading
        name={fragmentName(fragment, position)}
        noun="fragment"
        pending={pending}
        onRemove={onRemove}
      />

      <Field label="Name" hint="optional, and never sent to a model">
        <input
          value={fragment.name ?? ""}
          onChange={(event) =>
            onChange({ name: event.target.value || undefined })
          }
          disabled={pending}
        />
      </Field>

      {!isMarker ? (
        <Switch
          label="Sealed prompt"
          hint="Its text is sent only to an allowed linked application."
          checked={fragment.protected ?? false}
          pending={pending}
          onChange={(protectedPrompt) =>
            onChange({ protected: protectedPrompt })
          }
        />
      ) : null}

      {isMarker ? (
        <p className={styles.quiet}>
          This fragment is a marker. The app splices its own content in here,
          named <code>{fragment.marker}</code>, so it carries no text.
        </p>
      ) : (
        <Field label="Fragment text">
          <textarea
            rows={10}
            value={fragment.text}
            onChange={(event) => onChange({ text: event.target.value })}
            disabled={pending}
          />
        </Field>
      )}

      <FieldGroup legend="How it is sent">
        <FieldPair>
          <Field label="Speaks as">
            <select
              value={fragment.role ?? ""}
              onChange={(event) =>
                onChange({
                  role:
                    event.target.value === ""
                      ? undefined
                      : (event.target.value as PromptFragment["role"]),
                })
              }
              disabled={pending}
            >
              <option value="">Leave it to whatever reads the preset</option>
              {PROMPT_ROLES.map((role) => (
                <option key={role.value} value={role.value}>
                  {role.label}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Heading">
            <select
              value={fragment.groupId ?? ""}
              onChange={(event) =>
                onChange({ groupId: event.target.value || undefined })
              }
              disabled={pending}
            >
              <option value="">No heading</option>
              {groups.map((group) => (
                <option key={group.id} value={group.id}>
                  {group.name}
                </option>
              ))}
            </select>
          </Field>
        </FieldPair>
        <Switch
          label="Switched on"
          hint="A switched-off fragment stays in the preset and reaches no model."
          checked={fragment.enabled}
          pending={pending}
          onChange={(enabled) => onChange({ enabled })}
        />
      </FieldGroup>

      <FieldGroup legend="Where it goes">
        <FieldPair>
          <Field label="Placement">
            <select
              value={fragment.placement ?? ""}
              onChange={(event) =>
                onChange({
                  placement:
                    event.target.value === ""
                      ? undefined
                      : (event.target.value as PromptFragment["placement"]),
                })
              }
              disabled={pending}
            >
              <option value="">Leave it to whatever reads the preset</option>
              {PLACEMENTS.map((placement) => (
                <option key={placement.value} value={placement.value}>
                  {placement.label}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Depth" hint="messages back from the most recent">
            <input
              type="number"
              value={fragment.depth ?? ""}
              onChange={(event) =>
                onChange({
                  depth:
                    event.target.value === ""
                      ? undefined
                      : Number(event.target.value),
                })
              }
              disabled={pending}
            />
          </Field>
        </FieldPair>
      </FieldGroup>
    </ItemFields>
  );
}

export function SettingGroupEditor({
  settings,
  pending,
  onChange,
}: {
  settings: PresetSetting[];
  pending: boolean;
  onChange: (settings: PresetSetting[]) => void;
}) {
  return (
    <div className={styles.settings}>
      {settings.length === 0 ? (
        <p className={styles.quiet}>
          This group has no settings yet. Add the names your app reads.
        </p>
      ) : null}
      {settings.map((setting, index) => (
        <SettingRow
          key={setting.id ?? setting.name}
          setting={setting}
          pending={pending}
          onChange={(changes) => onChange(replaceAt(settings, index, changes))}
          onRemove={() => onChange(without(settings, index))}
        />
      ))}
      <NewSetting
        pending={pending}
        onAdd={(setting) => onChange([...settings, setting])}
      />
    </div>
  );
}

function SettingRow({
  setting,
  pending,
  onChange,
  onRemove,
}: {
  setting: PresetSetting;
  pending: boolean;
  onChange: (changes: Partial<PresetSetting>) => void;
  onRemove: () => void;
}) {
  const supplied = setting.value != null;
  const slot = nameSlot(setting.name);
  return (
    <div className={styles.setting} data-unset={supplied ? undefined : true}>
      <div className={styles.settingName}>
        <span>{slot.name}</span>
        <small>
          {slot.rank === "unrecognised" ? null : <code>{setting.name}</code>}
          {setting.type.replace("_", " ")}
        </small>
      </div>
      <div className={styles.settingValue}>
        <ValueField
          type={setting.type}
          choices={setting.choices}
          value={setting.value}
          label={setting.name}
          pending={pending}
          onChange={(value) => onChange({ value })}
        />
      </div>
      <div className={styles.settingActions}>
        {supplied ? (
          <button
            type="button"
            onClick={() => onChange({ value: undefined })}
            disabled={pending}
          >
            Leave it out
          </button>
        ) : (
          <button
            type="button"
            onClick={() => onChange({ value: emptyValue(setting.type) })}
            disabled={pending}
          >
            Fill it in
          </button>
        )}
        <button
          type="button"
          className={styles.removeSetting}
          onClick={onRemove}
          disabled={pending}
          aria-label={`Remove ${setting.name}`}
        >
          <Trash2 size={14} aria-hidden="true" />
        </button>
      </div>
    </div>
  );
}

function NewSetting({
  pending,
  onAdd,
}: {
  pending: boolean;
  onAdd: (setting: PresetSetting) => void;
}) {
  const [name, setName] = useState("");
  const [type, setType] = useState<PresetSetting["type"]>("number");
  return (
    <div className={styles.addSetting}>
      <input
        aria-label="A new setting's name"
        placeholder="The name your app reads"
        value={name}
        onChange={(event) => setName(event.target.value)}
        disabled={pending}
      />
      <select
        aria-label="What the new setting holds"
        value={type}
        onChange={(event) =>
          setType(event.target.value as PresetSetting["type"])
        }
        disabled={pending}
      >
        <option value="number">A number</option>
        <option value="boolean">Yes or no</option>
        <option value="text">Text</option>
        <option value="string_list">A list of strings</option>
      </select>
      <button
        type="button"
        onClick={() => {
          if (name.trim() === "") return;
          onAdd({ name: name.trim(), type });
          setName("");
        }}
        disabled={pending || name.trim() === ""}
      >
        <Plus size={16} aria-hidden="true" />
        Add setting
      </button>
    </div>
  );
}

function emptyValue(type: PresetSetting["type"]): TypedValue {
  switch (type) {
    case "number":
      return { number: 0 };
    case "boolean":
      return { boolean: false };
    case "string_list":
      return { strings: [] };
    default:
      return { text: "" };
  }
}

function ValueField({
  type,
  choices,
  value,
  label,
  pending,
  onChange,
}: {
  type: PresetSetting["type"];
  choices?: string[];
  value: TypedValue | undefined;
  label: string;
  pending: boolean;
  onChange: (value: TypedValue) => void;
}) {
  if (value == null) {
    return <p className={styles.unset}>Nobody has filled this in.</p>;
  }
  if (type === "boolean") {
    return (
      <Switch
        label={value.boolean ? "Yes" : "No"}
        checked={value.boolean ?? false}
        pending={pending}
        onChange={(boolean) => onChange({ boolean })}
      />
    );
  }
  if (type === "number") {
    return (
      <input
        type="number"
        aria-label={label}
        value={value.number ?? ""}
        onChange={(event) =>
          onChange({
            number: event.target.value === "" ? 0 : Number(event.target.value),
          })
        }
        disabled={pending}
      />
    );
  }
  if (type === "string_list") {
    return (
      <textarea
        aria-label={`${label}, one per line`}
        rows={3}
        value={writeLines(value.strings)}
        onChange={(event) =>
          onChange({ strings: readLines(event.target.value) })
        }
        disabled={pending}
      />
    );
  }
  if (choices && choices.length > 0) {
    return (
      <select
        aria-label={label}
        value={value.text ?? ""}
        onChange={(event) => onChange({ text: event.target.value })}
        disabled={pending}
      >
        {choices.map((choice) => (
          <option key={choice} value={choice}>
            {choice}
          </option>
        ))}
      </select>
    );
  }
  return (
    <input
      aria-label={label}
      value={value.text ?? ""}
      onChange={(event) => onChange({ text: event.target.value })}
      disabled={pending}
    />
  );
}

export function variableName(
  variable: PresetVariable,
  position: number,
): string {
  if (variable.label?.trim()) return variable.label;
  if (variable.name.trim()) return variable.name;
  return `Variable ${position + 1}`;
}

export function VariableSchemaEditor({
  variables,
  pending,
  onChange,
}: {
  variables: PresetVariable[];
  pending: boolean;
  onChange: (variables: PresetVariable[]) => void;
}) {
  const [selected, setSelected] = useState(0);
  const current = variables[selected];

  return (
    <CollectionEditor
      noun="variable"
      emptyMessage="This preset asks a reader for nothing yet."
      pending={pending}
      selected={selected}
      onSelect={setSelected}
      onAdd={() => onChange([...variables, { name: "", widget: "switch" }])}
      rows={variables.map((variable, index) => ({
        name: variableName(variable, index),
        detail: variable.name,
        search: [
          variable.name,
          variable.label ?? "",
          variable.description ?? "",
        ]
          .join(" ")
          .toLowerCase(),
      }))}
    >
      {current ? (
        <VariableFields
          variable={current}
          position={selected}
          pending={pending}
          onChange={(changes) =>
            onChange(replaceAt(variables, selected, changes))
          }
          onRemove={() => {
            onChange(without(variables, selected));
            setSelected(Math.max(0, Math.min(selected, variables.length - 2)));
          }}
        />
      ) : (
        <NothingChosen>
          Choose a variable to edit it, or add the first one.
        </NothingChosen>
      )}
    </CollectionEditor>
  );
}

function VariableFields({
  variable,
  position,
  pending,
  onChange,
  onRemove,
}: {
  variable: PresetVariable;
  position: number;
  pending: boolean;
  onChange: (changes: Partial<PresetVariable>) => void;
  onRemove: () => void;
}) {
  const options = variable.options ?? [];
  const range = variable.range ?? {};
  const listed =
    variable.widget === "select" || variable.widget === "multiselect";
  const numeric = variable.widget === "number" || variable.widget === "slider";
  return (
    <ItemFields>
      <ItemHeading
        name={variableName(variable, position)}
        noun="variable"
        pending={pending}
        onRemove={onRemove}
      />

      <FieldPair>
        <Field label="Name" hint="what the fragments refer to it by">
          <input
            value={variable.name}
            onChange={(event) => onChange({ name: event.target.value })}
            disabled={pending}
          />
        </Field>
        <Field label="Filled in with">
          <select
            value={variable.widget}
            onChange={(event) =>
              onChange({
                widget: event.target.value as PresetVariable["widget"],
              })
            }
            disabled={pending}
          >
            {WIDGETS.map((widget) => (
              <option key={widget.value} value={widget.value}>
                {widget.label}
              </option>
            ))}
          </select>
        </Field>
      </FieldPair>

      <Field label="Label" hint="what a reader sees above the control">
        <input
          value={variable.label ?? ""}
          onChange={(event) =>
            onChange({ label: event.target.value || undefined })
          }
          disabled={pending}
        />
      </Field>

      <Field label="Description" hint="the line under it">
        <textarea
          rows={3}
          value={variable.description ?? ""}
          onChange={(event) =>
            onChange({ description: event.target.value || undefined })
          }
          disabled={pending}
        />
      </Field>

      {listed ? (
        <FieldGroup legend="What a reader picks from">
          {options.map((option, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: Options stay ordered and hold no local state.
            <div className={styles.option} key={index}>
              <input
                aria-label={`Wording for choice ${index + 1}`}
                placeholder="Wording"
                value={option.label}
                onChange={(event) =>
                  onChange({
                    options: replaceAt(options, index, {
                      label: event.target.value,
                    }),
                  })
                }
                disabled={pending}
              />
              <input
                aria-label={`Value for choice ${index + 1}`}
                placeholder="What reaches the prompt"
                value={option.value}
                onChange={(event) =>
                  onChange({
                    options: replaceAt(options, index, {
                      value: event.target.value,
                    }),
                  })
                }
                disabled={pending}
              />
              <button
                type="button"
                className={styles.removeSetting}
                onClick={() => onChange({ options: without(options, index) })}
                disabled={pending}
                aria-label={`Remove choice ${index + 1}`}
              >
                <Trash2 size={14} aria-hidden="true" />
              </button>
            </div>
          ))}
          <button
            type="button"
            className={styles.addOption}
            onClick={() =>
              onChange({ options: [...options, { label: "", value: "" }] })
            }
            disabled={pending}
          >
            <Plus size={16} aria-hidden="true" />
            Add choice
          </button>
          {variable.widget === "multiselect" ? (
            <Field
              label="Separator"
              hint="what joins the chosen values in the prompt"
            >
              <input
                value={variable.separator ?? ""}
                onChange={(event) =>
                  onChange({ separator: event.target.value || undefined })
                }
                disabled={pending}
              />
            </Field>
          ) : null}
        </FieldGroup>
      ) : null}

      {numeric ? (
        <FieldGroup legend="What it accepts">
          <FieldPair>
            <Field label="Lowest">
              <input
                type="number"
                value={range.min ?? ""}
                onChange={(event) =>
                  onChange({
                    range: {
                      ...range,
                      min:
                        event.target.value === ""
                          ? undefined
                          : Number(event.target.value),
                    },
                  })
                }
                disabled={pending}
              />
            </Field>
            <Field label="Highest">
              <input
                type="number"
                value={range.max ?? ""}
                onChange={(event) =>
                  onChange({
                    range: {
                      ...range,
                      max:
                        event.target.value === ""
                          ? undefined
                          : Number(event.target.value),
                    },
                  })
                }
                disabled={pending}
              />
            </Field>
          </FieldPair>
          <Field label="Step">
            <input
              type="number"
              value={range.step ?? ""}
              onChange={(event) =>
                onChange({
                  range: {
                    ...range,
                    step:
                      event.target.value === ""
                        ? undefined
                        : Number(event.target.value),
                  },
                })
              }
              disabled={pending}
            />
          </Field>
        </FieldGroup>
      ) : null}
    </ItemFields>
  );
}

export function scriptName(script: RegexScript, position: number): string {
  if (script.name?.trim()) return script.name;
  if (script.find.trim()) return script.find;
  return `Script ${position + 1}`;
}

export function ScriptListEditor({
  scripts,
  pending,
  onChange,
}: {
  scripts: RegexScript[];
  pending: boolean;
  onChange: (scripts: RegexScript[]) => void;
}) {
  const [selected, setSelected] = useState(0);
  const current = scripts[selected];

  return (
    <CollectionEditor
      noun="script"
      emptyMessage="This preset changes nothing yet."
      pending={pending}
      selected={selected}
      onSelect={setSelected}
      onAdd={() =>
        onChange([...scripts, { find: "", replace: "", enabled: true }])
      }
      rows={scripts.map((script, index) => ({
        name: scriptName(script, index),
        detail: script.find,
        off: !script.enabled,
        search: [script.name ?? "", script.find, script.replace]
          .join(" ")
          .toLowerCase(),
      }))}
    >
      {current ? (
        <ScriptFields
          script={current}
          position={selected}
          pending={pending}
          onChange={(changes) =>
            onChange(replaceAt(scripts, selected, changes))
          }
          onRemove={() => {
            onChange(without(scripts, selected));
            setSelected(Math.max(0, Math.min(selected, scripts.length - 2)));
          }}
        />
      ) : (
        <NothingChosen>
          Choose a script to edit it, or add the first one.
        </NothingChosen>
      )}
    </CollectionEditor>
  );
}

function ScriptFields({
  script,
  position,
  pending,
  onChange,
  onRemove,
}: {
  script: RegexScript;
  position: number;
  pending: boolean;
  onChange: (changes: Partial<RegexScript>) => void;
  onRemove: () => void;
}) {
  const targets = script.targets ?? [];
  const affects = script.affects ?? [];
  return (
    <ItemFields>
      <ItemHeading
        name={scriptName(script, position)}
        noun="script"
        pending={pending}
        onRemove={onRemove}
      />

      <Field label="Name" hint="optional">
        <input
          value={script.name ?? ""}
          onChange={(event) =>
            onChange({ name: event.target.value || undefined })
          }
          disabled={pending}
        />
      </Field>

      <FieldPair>
        <Field label="Find">
          <input
            className={styles.pattern}
            value={script.find}
            onChange={(event) => onChange({ find: event.target.value })}
            disabled={pending}
          />
        </Field>
        <Field label="Flags" hint="g for every match, i to ignore case">
          <input
            className={styles.pattern}
            value={script.flags ?? ""}
            onChange={(event) =>
              onChange({ flags: event.target.value || undefined })
            }
            disabled={pending}
          />
        </Field>
      </FieldPair>

      <Field label="Replace it with">
        <textarea
          rows={4}
          value={script.replace}
          onChange={(event) => onChange({ replace: event.target.value })}
          disabled={pending}
        />
      </Field>

      <FieldGroup legend="What it runs over">
        {SCRIPT_TARGETS.map((target) => (
          <Switch
            key={target.value}
            label={target.label}
            checked={targets.includes(target.value)}
            pending={pending}
            onChange={(on) =>
              onChange({ targets: toggle(targets, target.value, on) })
            }
          />
        ))}
      </FieldGroup>

      <FieldGroup legend="What it changes">
        {SCRIPT_EFFECTS.map((effect) => (
          <Switch
            key={effect.value}
            label={effect.label}
            checked={affects.includes(effect.value)}
            pending={pending}
            onChange={(on) =>
              onChange({ affects: toggle(affects, effect.value, on) })
            }
          />
        ))}
      </FieldGroup>

      <FieldGroup legend="How far back it reaches">
        <FieldPair>
          <Field label="Nearest message" hint="counted from the most recent">
            <input
              type="number"
              value={script.minDepth ?? ""}
              onChange={(event) =>
                onChange({
                  minDepth:
                    event.target.value === ""
                      ? undefined
                      : Number(event.target.value),
                })
              }
              disabled={pending}
            />
          </Field>
          <Field label="Furthest message">
            <input
              type="number"
              value={script.maxDepth ?? ""}
              onChange={(event) =>
                onChange({
                  maxDepth:
                    event.target.value === ""
                      ? undefined
                      : Number(event.target.value),
                })
              }
              disabled={pending}
            />
          </Field>
        </FieldPair>
        <Switch
          label="Switched on"
          hint="A switched-off script stays in the preset and changes nothing."
          checked={script.enabled}
          pending={pending}
          onChange={(enabled) => onChange({ enabled })}
        />
        <Switch
          label="Run it again when a message is edited"
          checked={script.runOnEdit ?? false}
          pending={pending}
          onChange={(runOnEdit) => onChange({ runOnEdit })}
        />
      </FieldGroup>

      <Field label="Trim" hint="text cut out of the match, one per line">
        <textarea
          rows={2}
          value={writeLines(script.trim)}
          onChange={(event) =>
            onChange({ trim: readLines(event.target.value) })
          }
          disabled={pending}
        />
      </Field>
    </ItemFields>
  );
}

function toggle<T>(values: T[], value: T, on: boolean): T[] {
  if (on) return values.includes(value) ? values : [...values, value];
  return values.filter((held) => held !== value);
}
