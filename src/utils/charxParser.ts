import { unzipSync } from 'fflate';
import type { ParsedCharacterData } from './pngParser';

// ── Types ──────────────────────────────────────────────────────────────────

export interface BundledRegexScript {
  name: string;
  find_regex: string;
  replace_string: string;
  flags: string;
  placement: string[];
  scope: string;
  scope_id: string | null;
  target: string;
  min_depth: number | null;
  max_depth: number | null;
  trim_strings: string[];
  run_on_edit: boolean;
  substitute_macros: string;
  disabled: boolean;
  sort_order: number;
  description: string;
  metadata: Record<string, any>;
}

export interface LumiverseModules {
  version: number;
  has_nsfw_expressions?: boolean;
  expressions?: {
    enabled: boolean;
    defaultExpression: string;
    mappings: Record<string, string>;
  };
  alternate_fields?: Record<string, Array<{ id: string; label: string; content: string }>>;
  alternate_avatars?: Array<{ id: string; label: string; path: string }>;
  world_books?: Array<{ name: string; description?: string; entries?: any[] }>;
  regex_scripts?: BundledRegexScript[];
}

export interface CharxAsset {
  blob: Blob;
  url: string;
}

export interface CharxExpressionAsset extends CharxAsset {
  label: string;
}

export interface CharxAltAvatarAsset extends CharxAsset {
  id: string;
  label: string;
}

export interface CharxGalleryAsset extends CharxAsset {
  id: string;
}

export interface ParsedCharx {
  card: ParsedCharacterData;
  primaryAvatar: CharxAsset | null;
  alternateAvatars: CharxAltAvatarAsset[];
  expressions: CharxExpressionAsset[];
  gallery: CharxGalleryAsset[];
  lumiverseModules: LumiverseModules | null;
  rawFile: File;
}

// ── Constants ──────────────────────────────────────────────────────────────

const IMAGE_EXTENSIONS = /\.(png|jpg|jpeg|gif|webp|avif|bmp|svg)$/i;

const MIME_MAP: Record<string, string> = {
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  gif: 'image/gif',
  webp: 'image/webp',
  avif: 'image/avif',
  bmp: 'image/bmp',
  svg: 'image/svg+xml',
};

function mimeFromPath(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() || 'png';
  return MIME_MAP[ext] || 'image/png';
}

function blobFromBytes(bytes: Uint8Array, path: string): Blob {
  return new Blob([bytes], { type: mimeFromPath(path) });
}

// ── Parser ─────────────────────────────────────────────────────────────────

export async function parseCharxFile(file: File): Promise<ParsedCharx | null> {
  try {
    const buffer = new Uint8Array(await file.arrayBuffer());

    // Validate ZIP magic
    if (buffer.length < 4 || buffer[0] !== 0x50 || buffer[1] !== 0x4b || buffer[2] !== 0x03 || buffer[3] !== 0x04) {
      return null;
    }

    const unzipped = unzipSync(buffer, {
      filter: (entry) =>
        entry.name === 'card.json' ||
        entry.name === 'lumiverse_modules.json' ||
        IMAGE_EXTENSIONS.test(entry.name),
    });

    // Parse card.json
    const cardBytes = unzipped['card.json'];
    if (!cardBytes) return null;

    const json = JSON.parse(new TextDecoder().decode(cardBytes));
    const data = json.data ?? json;

    const card: ParsedCharacterData = {
      name: data.name ?? '',
      description: data.description ?? '',
      personality: data.personality ?? '',
      scenario: data.scenario ?? '',
      first_mes: data.first_mes ?? '',
      mes_example: data.mes_example ?? '',
      creator: data.creator ?? '',
      creator_notes: data.creator_notes ?? '',
      tags: Array.isArray(data.tags) ? data.tags : [],
      system_prompt: data.system_prompt ?? '',
      post_history_instructions: data.post_history_instructions ?? '',
      alternate_greetings: Array.isArray(data.alternate_greetings) ? data.alternate_greetings : [],
      character_version: data.character_version ?? '',
      _raw: data,
    };

    // Parse lumiverse_modules.json
    let lumiverseModules: LumiverseModules | null = null;
    const modulesBytes = unzipped['lumiverse_modules.json'];
    if (modulesBytes) {
      try {
        const parsed = JSON.parse(new TextDecoder().decode(modulesBytes));
        if (parsed && typeof parsed === 'object' && typeof parsed.version === 'number') {
          lumiverseModules = parsed;
        }
      } catch { /* skip */ }
    }

    // Collect images by archive path
    const imagePaths = Object.keys(unzipped).filter(
      (p) => p !== 'card.json' && p !== 'lumiverse_modules.json' && IMAGE_EXTENSIONS.test(p),
    );

    let primaryAvatar: CharxAsset | null = null;
    const alternateAvatars: CharxAltAvatarAsset[] = [];
    const expressions: CharxExpressionAsset[] = [];
    const gallery: CharxGalleryAsset[] = [];

    for (const archivePath of imagePaths) {
      const bytes = unzipped[archivePath];
      if (!bytes || bytes.length === 0) continue;

      const blob = blobFromBytes(bytes, archivePath);
      const url = URL.createObjectURL(blob);
      const basename = archivePath.split('/').pop()?.replace(/\.[^.]+$/, '') || '';

      if (archivePath.startsWith('assets/icon/image/') && basename.startsWith('main')) {
        primaryAvatar = { blob, url };
      } else if (archivePath.startsWith('assets/icon/image/')) {
        let label = basename;
        if (lumiverseModules?.alternate_avatars) {
          const match = lumiverseModules.alternate_avatars.find((a) => a.path === archivePath);
          if (match) label = match.label;
        }
        alternateAvatars.push({ id: basename, label, blob, url });
      } else if (archivePath.startsWith('assets/other/image/expr_')) {
        const label = basename.replace(/^expr_/, '');
        expressions.push({ label, blob, url });
      } else if (archivePath.startsWith('assets/other/image/gallery_')) {
        gallery.push({ id: basename, blob, url });
      } else if (!primaryAvatar) {
        // Fallback: first unclassified image as avatar
        primaryAvatar = { blob, url };
      } else {
        gallery.push({ id: basename, blob, url });
      }
    }

    return {
      card,
      primaryAvatar,
      alternateAvatars,
      expressions,
      gallery,
      lumiverseModules,
      rawFile: file,
    };
  } catch (err) {
    console.warn('[LumiHub] Failed to parse .charx file:', err);
    return null;
  }
}

/** Revokes all object URLs created during parsing to free memory. */
export function revokeCharxUrls(parsed: ParsedCharx): void {
  if (parsed.primaryAvatar) URL.revokeObjectURL(parsed.primaryAvatar.url);
  for (const a of parsed.alternateAvatars) URL.revokeObjectURL(a.url);
  for (const e of parsed.expressions) URL.revokeObjectURL(e.url);
  for (const g of parsed.gallery) URL.revokeObjectURL(g.url);
}
