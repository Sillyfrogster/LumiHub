import { z } from 'zod';
import { FILE_SIZE_LIMITS } from '../utils/constants.ts';

const accentSchema = z.object({
  h: z.number().min(0).max(360),
  s: z.number().min(0).max(100),
  l: z.number().min(0).max(100),
});

const baseColorsSchema = z.record(z.string()).optional();

export const themeConfigSchema = z.object({
  id: z.string().optional(),
  name: z.string().min(1, 'Theme name is required'),
  mode: z.enum(['light', 'dark', 'system']),
  accent: accentSchema,
  statusColors: z.record(z.string()).optional(),
  baseColors: baseColorsSchema,
  baseColorsByMode: z.object({
    dark: baseColorsSchema,
    light: baseColorsSchema,
  }).partial().optional(),
  radiusScale: z.number().default(1),
  enableGlass: z.boolean().default(false),
  fontScale: z.number().default(1),
  uiScale: z.number().optional(),
  characterAware: z.boolean().optional(),
}).passthrough();

const customCssObjectSchema = z.object({
  bundleId: z.string().nullable().optional(),
  css: z.string(),
}).passthrough();

const themeWrapperSchema = z.object({
  type: z.literal('lumiverse_theme'),
  schemaVersion: z.number().int().positive().default(1),
  theme: themeConfigSchema,
  customCss: z.union([z.string(), customCssObjectSchema]).optional(),
  compatibility: z.record(z.any()).default({}),
}).passthrough();

const exportedThemePackageSchema = z.object({
  format: z.number().int().positive(),
  name: z.string().optional(),
  author: z.string().optional(),
  description: z.string().optional(),
  createdAt: z.number().optional(),
  bundleId: z.string().nullable().optional(),
  theme: themeConfigSchema,
  globalCSS: z.string().optional(),
  components: z.record(z.any()).optional(),
  assets: z.array(z.any()).optional(),
}).passthrough();

export type NormalizedThemeImport = {
  config: Record<string, any>;
  schemaVersion: number;
  compatibility: Record<string, any>;
  customCss: string | null;
  assetBundleId: string | null;
};

function extractCustomCss(value: unknown): string | null {
  if (typeof value === 'string') return value;
  if (value && typeof value === 'object' && typeof (value as { css?: unknown }).css === 'string') {
    return (value as { css: string }).css;
  }
  return null;
}

function validateCustomCss(css: string | null): string | null {
  if (css === null) return null;
  const bytes = new TextEncoder().encode(css).byteLength;
  if (bytes > FILE_SIZE_LIMITS.THEME) {
    throw new Error('Custom CSS exceeds 2MB limit');
  }
  return css;
}

export function normalizeThemeImport(input: unknown): NormalizedThemeImport {
  const wrapped = themeWrapperSchema.safeParse(input);
  if (wrapped.success) {
    return {
      config: wrapped.data.theme,
      schemaVersion: wrapped.data.schemaVersion,
      compatibility: wrapped.data.compatibility,
      customCss: validateCustomCss(extractCustomCss(wrapped.data.customCss)),
      assetBundleId: null,
    };
  }

  const exported = exportedThemePackageSchema.safeParse(input);
  if (exported.success) {
    return {
      config: exported.data.theme,
      schemaVersion: exported.data.format,
      compatibility: {},
      customCss: validateCustomCss(exported.data.globalCSS ?? null),
      assetBundleId: exported.data.bundleId ?? null,
    };
  }

  const bare = themeConfigSchema.safeParse(input);
  if (bare.success) {
    return {
      config: bare.data,
      schemaVersion: 1,
      compatibility: {},
      customCss: null,
      assetBundleId: null,
    };
  }

  throw new Error('Invalid Lumiverse theme payload');
}

export function buildThemeExport(params: {
  config: Record<string, any>;
  schemaVersion: number;
  compatibility?: Record<string, any>;
  customCss?: string | null;
  assetBundleId?: string | null;
}) {
  return {
    format: params.schemaVersion,
    name: String(params.config.name ?? 'Untitled Theme'),
    author: '',
    description: '',
    bundleId: params.assetBundleId ?? null,
    theme: params.config,
    globalCSS: params.customCss ?? '',
    components: {},
    assets: [],
    compatibility: params.compatibility ?? {},
  };
}
