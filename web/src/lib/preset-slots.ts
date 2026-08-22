/**
 * The names a preset's settings and nudges are read under. Nothing here
 * reaches a download, and a key with no entry is shown as the file wrote it.
 */

export type SlotRank = "lead" | "supporting" | "unrecognised";

export type NamedSlot = {
  /** The key the file uses, whether or not Illarin knows it. */
  key: string;
  name: string;
  note?: string;
  rank: SlotRank;
};

type SlotEntry = {
  keys: string[];
  name: string;
  note?: string;
  /** Set on the handful a reader checks to see whether a preset suits them. */
  lead?: true;
};

const SLOTS: SlotEntry[] = [
  { keys: ["accent"], name: "Accent colour", lead: true },
  { keys: ["mode"], name: "Preferred mode", lead: true },
  { keys: ["radiusScale"], name: "Corner scale" },
  { keys: ["enableGlass"], name: "Glass surfaces" },
  { keys: ["fontScale", "font_scale"], name: "Type scale" },
  { keys: ["uiScale"], name: "Interface scale" },
  { keys: ["characterAware"], name: "Character-aware colours" },
  { keys: ["blur_strength"], name: "Backdrop blur" },
  { keys: ["shadow_width"], name: "Shadow width" },
  { keys: ["avatar_style"], name: "Avatar shape" },
  { keys: ["chat_display"], name: "Chat layout" },
  { keys: ["chat_width"], name: "Chat width" },
  { keys: ["fast_ui_mode"], name: "Fast interface mode" },
  { keys: ["waifuMode"], name: "Visual novel mode" },
  { keys: ["noShadows"], name: "Remove shadows" },
  { keys: ["timer_enabled"], name: "Message timer" },
  { keys: ["timestamps_enabled"], name: "Timestamps" },
  { keys: ["timestamp_model_icon"], name: "Model icon by timestamp" },
  { keys: ["mesIDDisplay_enabled"], name: "Message IDs" },
  { keys: ["hideChatAvatars_enabled"], name: "Hide chat avatars" },
  { keys: ["message_token_count_enabled"], name: "Message token counts" },
  { keys: ["expand_message_actions"], name: "Expanded message actions" },
  { keys: ["enableZenSliders"], name: "Compact sliders" },
  { keys: ["enableLabMode"], name: "Lab mode" },
  { keys: ["hotswap_enabled"], name: "Character hotswap" },
  { keys: ["bogus_folders"], name: "Folder-style groups" },
  { keys: ["reduced_motion"], name: "Reduced motion" },
  { keys: ["compact_input_area"], name: "Compact composer" },
  {
    keys: ["temperature"],
    name: "Temperature",
    note: "Higher values wander further from the model's likeliest next word.",
    lead: true,
  },
  {
    keys: ["openai_max_context", "contextSize"],
    name: "Context size",
    note: "How much of the conversation is sent with each reply.",
    lead: true,
  },
  {
    keys: ["openai_max_tokens", "maxTokens"],
    name: "Maximum reply length",
    note: "The longest reply the model is allowed to write.",
    lead: true,
  },
  {
    keys: ["top_p"],
    name: "Top P",
    note: "Keeps the likeliest words until their odds add up to this much.",
    lead: true,
  },
  {
    keys: ["top_k"],
    name: "Top K",
    note: "Keeps only this many of the likeliest words.",
    lead: true,
  },
  {
    keys: ["min_p"],
    name: "Min P",
    note: "Drops words whose odds fall this far under the best one's.",
    lead: true,
  },
  {
    keys: ["top_a"],
    name: "Top A",
    note: "Drops words whose odds fall far under the best one's.",
  },
  {
    keys: ["repetition_penalty"],
    name: "Repetition penalty",
    note: "Pushes the model off words it has already used.",
  },
  {
    keys: ["frequency_penalty"],
    name: "Frequency penalty",
    note: "Pushes harder the more often a word has been used.",
  },
  {
    keys: ["presence_penalty"],
    name: "Presence penalty",
    note: "Pushes off any word that has been used at all.",
  },
  {
    keys: ["seed"],
    name: "Seed",
    note: "The same seed and the same request give the same reply.",
  },
  { keys: ["n"], name: "Replies per request" },
  {
    keys: ["max_context_unlocked"],
    name: "Unlock the context limit",
    note: "Lets the context size go past what the app offers.",
  },
  {
    keys: ["bias_preset_selected"],
    name: "Logit bias set",
    note: "The named set of word biases this preset asks for.",
  },
  {
    keys: ["customStopStrings"],
    name: "Stop strings",
    note: "The model stops writing when it reaches one of these.",
  },
  { keys: ["collapseMessages"], name: "Collapse consecutive messages" },
  { keys: ["trimIncompleteWords"], name: "Trim a cut-off word" },
  {
    keys: ["enabled"],
    name: "Use these sampler settings",
    note: "The preset's own values are sent rather than the app's.",
  },
  {
    keys: ["stream_openai", "streaming"],
    name: "Stream the reply",
    note: "The reply arrives word by word rather than all at once.",
  },
  { keys: ["use_sysprompt", "useSystemPrompt"], name: "Send a system prompt" },
  {
    keys: ["squash_system_messages", "squashSystemMessages"],
    name: "Merge system messages",
    note: "A run of system messages is sent as one.",
  },
  {
    keys: ["names_behavior", "namesBehavior"],
    name: "Character names in messages",
    note: "How a speaker's name is attached to what they say.",
  },
  {
    keys: ["assistant_prefill", "assistantPrefill"],
    name: "Assistant prefill",
    note: "Text the model's reply is made to start with.",
  },
  {
    keys: ["assistant_impersonation", "assistantImpersonation"],
    name: "Impersonation prefill",
    note: "Text a reply written as the user is made to start with.",
  },
  {
    keys: ["reasoningPrefill"],
    name: "Reasoning prefill",
    note: "Text the model's reasoning is made to start with.",
  },
  {
    keys: ["continue_prefill", "continuePrefill"],
    name: "Continue from the last reply",
  },
  {
    keys: ["continue_postfix", "continuePostfix"],
    name: "Continue joiner",
    note: "What goes between a cut-off reply and the rest of it.",
  },
  {
    keys: ["function_calling", "enableFunctionCalling"],
    name: "Function calling",
  },
  { keys: ["enable_web_search", "enableWebSearch"], name: "Web search" },
  {
    keys: ["media_inlining", "sendInlineMedia"],
    name: "Send images to the model",
  },
  { keys: ["inline_image_quality"], name: "Quality of images sent" },
  { keys: ["show_thoughts"], name: "Show the model's reasoning" },
  { keys: ["reasoning_effort"], name: "Reasoning effort" },
  { keys: ["verbosity"], name: "Verbosity" },
  { keys: ["request_images"], name: "Ask the model for images" },
  { keys: ["request_image_aspect_ratio"], name: "Shape of images asked for" },
  { keys: ["request_image_resolution"], name: "Size of images asked for" },
  {
    keys: ["includeUsage"],
    name: "Report token usage",
    note: "The app is told how many tokens each reply cost.",
  },
  {
    keys: ["impersonation_prompt", "impersonationPrompt"],
    name: "Write as the user",
  },
  { keys: ["new_chat_prompt", "newChatPrompt"], name: "Start of a chat" },
  {
    keys: ["new_group_chat_prompt", "newGroupChatPrompt"],
    name: "Start of a group chat",
  },
  { keys: ["new_example_chat_prompt"], name: "Start of the example messages" },
  {
    keys: ["continue_nudge_prompt", "continueNudge"],
    name: "Continue a cut-off reply",
  },
  {
    keys: ["group_nudge_prompt", "groupNudge"],
    name: "Whose turn it is in a group",
  },
  {
    keys: ["send_if_empty", "sendIfEmpty"],
    name: "Sent when the user says nothing",
  },
  {
    keys: ["emptySendNudge"],
    name: "Nudge when the user says nothing",
  },
  {
    keys: ["wi_format"],
    name: "How lorebook text is wrapped",
    note: "{0} stands for the text the entry contributes.",
  },
  { keys: ["scenario_format"], name: "How the scenario is wrapped" },
  { keys: ["personality_format"], name: "How the personality is wrapped" },
];

const BY_KEY = new Map(
  SLOTS.flatMap((entry) => entry.keys.map((key) => [plainKey(key), entry])),
);

function plainKey(key: string): string {
  return key.toLocaleLowerCase().replaceAll(/[^a-z0-9]/g, "");
}

export function nameSlot(key: string): NamedSlot {
  const entry = BY_KEY.get(plainKey(key));
  if (!entry) return { key, name: key, rank: "unrecognised" };
  return {
    key,
    name: entry.name,
    note: entry.note,
    rank: entry.lead ? "lead" : "supporting",
  };
}

const RANK_ORDER: Record<SlotRank, number> = {
  lead: 0,
  supporting: 1,
  unrecognised: 2,
};

/** The settings a group shows, in the order a reader wants them. */
export function orderSettings<T extends { name: string }>(
  settings: readonly T[],
): Array<T & { slot: NamedSlot }> {
  return settings
    .map((setting) => ({ ...setting, slot: nameSlot(setting.name) }))
    .sort(
      (one, other) => RANK_ORDER[one.slot.rank] - RANK_ORDER[other.slot.rank],
    );
}
