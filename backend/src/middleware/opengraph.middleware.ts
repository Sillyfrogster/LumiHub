import path from 'node:path';
import { AppDataSource } from '../db/connection.ts';
import { Character } from '../entities/Character.entity.ts';
import { CharacterImage } from '../entities/CharacterImage.entity.ts';
import { Worldbook } from '../entities/Worldbook.entity.ts';
import { Preset } from '../entities/Preset.entity.ts';
import { Theme } from '../entities/Theme.entity.ts';
import { env } from '../env.ts';
import { logger } from '../utils/logger.ts';
import type { MiddlewareHandler } from 'hono';

const INDEX_PATH = path.resolve('../dist/index.html');
let cachedIndexHtml: string | null = null;

async function getIndexHtml(): Promise<string> {
  if (cachedIndexHtml) return cachedIndexHtml;
  try {
    const file = Bun.file(INDEX_PATH);
    cachedIndexHtml = await file.text();
  } catch {
    cachedIndexHtml = '<!doctype html><html><head></head><body><div id="root"></div></body></html>';
  }
  return cachedIndexHtml;
}

function escapeHtml(str: string): string {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function truncate(str: string, len: number): string {
  if (str.length <= len) return str;
  return str.slice(0, len - 1) + '\u2026';
}

function buildImageUrl(imagePath: string | null): string | null {
  if (!imagePath) return null;
  const normalized = imagePath.replace(/\\/g, '/');
  const publicUrl = env.LUMIHUB_PUBLIC_URL;
  return `${publicUrl}/${normalized.startsWith('uploads/') ? normalized : `uploads/${normalized}`}`;
}

interface OGMeta {
  title: string;
  description: string;
  image?: string | null;
  type?: string;
  url: string;
}

function injectMeta(html: string, meta: OGMeta): string {
  const tags = [
    `<meta property="og:title" content="${escapeHtml(meta.title)}" />`,
    `<meta property="og:description" content="${escapeHtml(meta.description)}" />`,
    `<meta property="og:type" content="${meta.type || 'website'}" />`,
    `<meta property="og:url" content="${escapeHtml(meta.url)}" />`,
    `<meta property="og:site_name" content="LumiHub" />`,
    // Twitter card
    `<meta name="twitter:card" content="${meta.image ? 'summary_large_image' : 'summary'}" />`,
    `<meta name="twitter:title" content="${escapeHtml(meta.title)}" />`,
    `<meta name="twitter:description" content="${escapeHtml(meta.description)}" />`,
    // Standard meta
    `<meta name="description" content="${escapeHtml(meta.description)}" />`,
  ];

  if (meta.image) {
    tags.push(`<meta property="og:image" content="${escapeHtml(meta.image)}" />`);
    tags.push(`<meta name="twitter:image" content="${escapeHtml(meta.image)}" />`);
  }

  // Theme color for Discord embeds
  tags.push(`<meta name="theme-color" content="#d88c9a" />`);

  const titleTag = `<title>${escapeHtml(meta.title)}</title>`;

  return html
    .replace(/<title>.*?<\/title>/, titleTag)
    .replace('</head>', `${tags.join('\n    ')}\n  </head>`);
}

// ── Route handlers ─────────────────────────────────────────────────────────

async function characterMeta(id: string): Promise<OGMeta | null> {
  try {
    const repo = AppDataSource.getRepository(Character);
    const char = await repo.findOne({ where: { id }, relations: ['owner'] });
    if (!char) return null;

    const imageRepo = AppDataSource.getRepository(CharacterImage);
    const images = await imageRepo.find({ where: { character_id: id } });

    const expressions = images.filter((i) => i.image_type === 'expression');
    const gallery = images.filter((i) => i.image_type === 'gallery');
    const altAvatars = images.filter((i) => i.image_type === 'avatar_alt');

    // Count alternate fields
    const modules = char.extensions?.lumiverse_modules as Record<string, any> | undefined;
    const altFields = modules?.alternate_fields;
    const altFieldCount = altFields
      ? Object.values(altFields).reduce((sum: number, arr: any) => sum + (Array.isArray(arr) ? arr.length : 0), 0)
      : 0;

    // Count lorebook entries
    const lorebook = char.character_book as { entries?: unknown[] } | null;
    const lorebookCount = lorebook?.entries?.length ?? 0;

    // Build CTA description
    const creator = char.owner?.username || char.creator || 'Unknown';

    const features: string[] = [];
    if (lorebookCount > 0) features.push(`${lorebookCount} lorebook entr${lorebookCount === 1 ? 'y' : 'ies'}`);
    if (expressions.length > 0) features.push(`${expressions.length} expression image${expressions.length === 1 ? '' : 's'}`);
    if (altAvatars.length > 0) features.push(`${altAvatars.length} alternate avatar${altAvatars.length === 1 ? '' : 's'}`);
    if (altFieldCount > 0) features.push(`${altFieldCount} alternate field${altFieldCount === 1 ? '' : 's'}`);
    if (gallery.length > 0) features.push(`${gallery.length} gallery image${gallery.length === 1 ? '' : 's'}`);
    if (char.alternate_greetings?.length > 0) features.push(`${char.alternate_greetings.length} alternate greeting${char.alternate_greetings.length === 1 ? '' : 's'}`);

    // "only on LumiHub" for native uploads; neutral CTA for aggregated content
    const isNativeUpload = !!char.owner_id;
    const cta = isNativeUpload ? 'only on LumiHub!' : 'available on LumiHub.';

    let description: string;
    if (features.length > 0) {
      description = `${char.name} by ${creator} has ${features.join(', ')} — ${cta}`;
    } else {
      description = `${char.name} by ${creator} — browse and install directly on LumiHub!`;
    }

    return {
      title: `${char.name} - LumiHub`,
      description: truncate(description, 300),
      image: buildImageUrl(char.image_path),
      type: 'article',
      url: `${env.LUMIHUB_PUBLIC_URL}/characters/${id}`,
    };
  } catch (err) {
    logger.warn('[OG] Failed to build character meta:', err);
    return null;
  }
}

async function worldbookMeta(id: string): Promise<OGMeta | null> {
  try {
    const repo = AppDataSource.getRepository(Worldbook);
    const wb = await repo.findOne({ where: { id }, relations: ['owner'] });
    if (!wb) return null;

    const entries = wb.entries as { entries?: unknown[] } | Record<string, unknown>;
    const entryCount = Array.isArray((entries as any)?.entries)
      ? (entries as any).entries.length
      : (Array.isArray(entries) ? entries.length : Object.keys(entries).length);

    const creator = wb.owner?.username || 'Unknown';
    const desc = entryCount > 0
      ? `A worldbook by ${creator} with ${entryCount} entr${entryCount === 1 ? 'y' : 'ies'}. Expand your characters' knowledge on LumiHub!`
      : `A worldbook by ${creator}. Expand your characters' knowledge on LumiHub!`;

    return {
      title: `${wb.name} - Worldbook - LumiHub`,
      description: truncate(wb.description || desc, 300),
      image: buildImageUrl(wb.image_path),
      url: `${env.LUMIHUB_PUBLIC_URL}/worldbooks/${id}`,
    };
  } catch (err) {
    logger.warn('[OG] Failed to build worldbook meta:', err);
    return null;
  }
}

async function presetMeta(id: string): Promise<OGMeta | null> {
  try {
    const repo = AppDataSource.getRepository(Preset);
    const preset = await repo.findOne({ where: { id }, relations: ['owner'] });
    if (!preset) return null;

    const creator = preset.owner?.username || 'Unknown';
    const desc = preset.description
      ? truncate(preset.description, 200) + ' — Discover generation presets on LumiHub!'
      : `A generation preset by ${creator}. Fine-tune your AI outputs on LumiHub!`;

    return {
      title: `${preset.name} - Preset - LumiHub`,
      description: truncate(desc, 300),
      image: buildImageUrl(preset.image_path),
      url: `${env.LUMIHUB_PUBLIC_URL}/presets/${id}`,
    };
  } catch (err) {
    logger.warn('[OG] Failed to build preset meta:', err);
    return null;
  }
}

async function themeMeta(id: string): Promise<OGMeta | null> {
  try {
    const repo = AppDataSource.getRepository(Theme);
    const theme = await repo.findOne({ where: { id }, relations: ['owner'] });
    if (!theme) return null;

    const creator = theme.owner?.username || 'Unknown';
    const desc = theme.description
      ? truncate(theme.description, 200) + ' — Customize your Lumiverse with themes from LumiHub!'
      : `A UI theme by ${creator}. Customize your Lumiverse experience on LumiHub!`;

    return {
      title: `${theme.name} - Theme - LumiHub`,
      description: truncate(desc, 300),
      image: buildImageUrl(theme.image_path),
      url: `${env.LUMIHUB_PUBLIC_URL}/themes/${id}`,
    };
  } catch (err) {
    logger.warn('[OG] Failed to build theme meta:', err);
    return null;
  }
}

// ── Route patterns ─────────────────────────────────────────────────────────

const OG_ROUTES: Array<{ pattern: RegExp; handler: (id: string) => Promise<OGMeta | null> }> = [
  { pattern: /^\/characters\/([0-9a-f-]{36})$/i, handler: characterMeta },
  { pattern: /^\/worldbooks\/([0-9a-f-]{36})$/i, handler: worldbookMeta },
  { pattern: /^\/presets\/([0-9a-f-]{36})$/i, handler: presetMeta },
  { pattern: /^\/themes\/([0-9a-f-]{36})$/i, handler: themeMeta },
];

/**
 * OpenGraph middleware — intercepts content routes in production,
 * injects OG meta tags into index.html for social media crawlers.
 * Non-matching routes or failures fall through to the normal SPA serving.
 */
export const opengraphMiddleware: MiddlewareHandler = async (c, next) => {
  const urlPath = new URL(c.req.url).pathname;

  for (const route of OG_ROUTES) {
    const match = urlPath.match(route.pattern);
    if (match) {
      const id = match[1];
      const meta = await route.handler(id);
      if (meta) {
        const html = await getIndexHtml();
        const injected = injectMeta(html, meta);
        return c.html(injected);
      }
      break;
    }
  }

  await next();
};
