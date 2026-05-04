import { z } from 'zod';

const samplerOverridesSchema = z.object({
  enabled: z.boolean().default(true),
  maxTokens: z.number().nullable().default(null),
  contextSize: z.number().nullable().default(null),
  temperature: z.number().nullable().default(null),
  topP: z.number().nullable().default(null),
  minP: z.number().nullable().default(null),
  topK: z.number().nullable().default(null),
  frequencyPenalty: z.number().nullable().default(null),
  presencePenalty: z.number().nullable().default(null),
  repetitionPenalty: z.number().nullable().default(null),
  streaming: z.boolean().default(true),
}).passthrough();

const customBodySchema = z.object({
  enabled: z.boolean().default(false),
  rawJson: z.string().default('{}'),
}).passthrough();

const promptBehaviorSchema = z.object({
  continueNudge: z.string().default('[Continue your last message...]'),
  emptySendNudge: z.string().default('[Write the next reply only as {{char}}.]'),
  impersonationPrompt: z.string().default('[Write the next reply only as {{user}}. Don\'t give {{user}} a chat partner\'s reply.]'),
  groupNudge: z.string().default('[Write the next reply only as {{char}}.]'),
  newChatPrompt: z.string().default('[Start a new Chat]'),
  newGroupChatPrompt: z.string().default('[Start a new group chat. Group members: {{group}}]'),
  sendIfEmpty: z.string().default(''),
}).passthrough();

const completionSettingsSchema = z.object({
  assistantPrefill: z.string().default(''),
  assistantImpersonation: z.string().default(''),
  continuePrefill: z.boolean().default(false),
  continuePostfix: z.string().default(' '),
  namesBehavior: z.number().default(0),
  squashSystemMessages: z.boolean().default(false),
  useSystemPrompt: z.boolean().default(true),
  enableWebSearch: z.boolean().default(false),
  sendInlineMedia: z.boolean().default(true),
  enableFunctionCalling: z.boolean().default(true),
  includeUsage: z.boolean().default(false),
}).passthrough();

const advancedSettingsSchema = z.object({
  seed: z.number().default(-1),
  customStopStrings: z.array(z.string()).default([]),
  collapseMessages: z.boolean().default(false),
}).passthrough();

const presetSchema = z.object({
  id: z.string().optional(),
  name: z.string().min(1, 'Preset name is required'),
  description: z.string().default(''),
  schemaVersion: z.number().int().positive(),
  createdAt: z.number().optional(),
  updatedAt: z.number().optional(),
  blocks: z.array(z.record(z.any())),
  source: z.any().nullable().default(null),
  isDefault: z.boolean().default(false),
  samplerOverrides: samplerOverridesSchema.default({}),
  customBody: customBodySchema.default({}),
  promptBehavior: promptBehaviorSchema.default({}),
  completionSettings: completionSettingsSchema.default({}),
  advancedSettings: advancedSettingsSchema.default({}),
  modelProfiles: z.record(z.any()).default({}),
  lastProfileKey: z.string().nullable().default(null),
  promptVariables: z.record(z.record(z.union([z.string(), z.number()]))).default({}),
}).passthrough();

const presetWrapperSchema = z.object({
  type: z.literal('lumiverse_preset'),
  schemaVersion: z.number().int().positive().default(1),
  preset: presetSchema,
  compatibility: z.record(z.any()).default({}),
}).passthrough();

export type NormalizedPresetImport = {
  preset: Record<string, any>;
  schemaVersion: number;
  compatibility: Record<string, any>;
};

export function normalizePresetImport(input: unknown): NormalizedPresetImport {
  const wrapped = presetWrapperSchema.safeParse(input);
  if (wrapped.success) {
    return {
      preset: wrapped.data.preset,
      schemaVersion: wrapped.data.schemaVersion,
      compatibility: wrapped.data.compatibility,
    };
  }

  const bare = presetSchema.safeParse(input);
  if (bare.success) {
    return {
      preset: bare.data,
      schemaVersion: bare.data.schemaVersion,
      compatibility: {},
    };
  }

  throw new Error('Invalid Lumiverse preset payload');
}

export function buildPresetExport(params: {
  preset: Record<string, any>;
  schemaVersion: number;
  compatibility?: Record<string, any>;
}) {
  return {
    type: 'lumiverse_preset',
    schemaVersion: params.schemaVersion,
    preset: params.preset,
    compatibility: params.compatibility ?? {},
  };
}
