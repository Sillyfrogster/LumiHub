import path from 'node:path';
import { mkdir } from 'node:fs/promises';
import { unzipSync } from 'fflate';
import type { MiddlewareHandler } from 'hono';
import { characterDataSchema, type ValidatedCharacterData } from './upload.middleware.ts';
import { FILE_SIZE_LIMITS, UPLOAD_PATHS } from '../utils/constants.ts';
import { logger } from '../utils/logger.ts';

// ── Types ──────────────────────────────────────────────────────────────────

export interface CharxImageEntry {
  type: 'avatar' | 'avatar_alt' | 'expression' | 'gallery';
  label: string | null;
  filePath: string;
  mimeType: string;
  fileSize: number;
}

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
  regex_scripts?: BundledRegexScript[];
}

export type CharxEnv = {
  Variables: {
    characterData: ValidatedCharacterData;
    imagePath: string | undefined;
    charxImages: CharxImageEntry[];
    lumiverseModules: LumiverseModules | null;
  };
};

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

const ZIP_MAGIC = [0x50, 0x4b, 0x03, 0x04];

// ── Middleware ──────────────────────────────────────────────────────────────

export const charxUploadMiddleware: MiddlewareHandler<CharxEnv> = async (c, next) => {
  const formData = await c.req.formData().catch(() => null);
  if (!formData) {
    return c.json({ error: 'Bad Request', message: 'Expected multipart/form-data', statusCode: 400 }, 400);
  }

  const charxFile = formData.get('charx_file');
  if (!charxFile || !(charxFile instanceof File)) {
    return c.json({ error: 'Bad Request', message: '"charx_file" field is required', statusCode: 400 }, 400);
  }

  if (charxFile.size > FILE_SIZE_LIMITS.CHARX) {
    const limitMB = (FILE_SIZE_LIMITS.CHARX / 1024 / 1024).toFixed(0);
    return c.json({ error: 'Bad Request', message: `File exceeds ${limitMB} MB limit`, statusCode: 400 }, 400);
  }

  const buffer = new Uint8Array(await charxFile.arrayBuffer());

  // Validate ZIP magic bytes
  if (buffer.length < 4 || !ZIP_MAGIC.every((b, i) => buffer[i] === b)) {
    return c.json({ error: 'Bad Request', message: 'File is not a valid ZIP/.charx archive', statusCode: 400 }, 400);
  }

  // Extract archive
  let unzipped: Record<string, Uint8Array>;
  try {
    unzipped = unzipSync(buffer, {
      filter: (entry) =>
        entry.name === 'card.json' ||
        entry.name === 'lumiverse_modules.json' ||
        IMAGE_EXTENSIONS.test(entry.name),
    });
  } catch (err) {
    logger.error('Failed to extract .charx archive:', err);
    return c.json({ error: 'Bad Request', message: 'Failed to extract .charx archive', statusCode: 400 }, 400);
  }

  // Parse card.json
  const cardBytes = unzipped['card.json'];
  if (!cardBytes) {
    return c.json({ error: 'Bad Request', message: 'Archive missing required card.json', statusCode: 400 }, 400);
  }

  let cardJson: any;
  try {
    cardJson = JSON.parse(new TextDecoder().decode(cardBytes));
  } catch {
    return c.json({ error: 'Bad Request', message: 'card.json is not valid JSON', statusCode: 400 }, 400);
  }

  // Handle CCSv3 wrapped format
  const rawData = (cardJson.spec === 'chara_card_v2' || cardJson.spec === 'chara_card_v3') && cardJson.data
    ? cardJson.data
    : cardJson;

  const result = characterDataSchema.safeParse(rawData);
  if (!result.success) {
    return c.json({
      error: 'Validation Error',
      message: 'card.json failed validation',
      details: result.error.flatten().fieldErrors,
      statusCode: 400,
    }, 400);
  }

  c.set('characterData', result.data);

  // Parse lumiverse_modules.json
  let lumiverseModules: LumiverseModules | null = null;
  const modulesBytes = unzipped['lumiverse_modules.json'];
  if (modulesBytes) {
    try {
      const parsed = JSON.parse(new TextDecoder().decode(modulesBytes));
      if (parsed && typeof parsed === 'object' && typeof parsed.version === 'number') {
        lumiverseModules = parsed as LumiverseModules;
      }
    } catch { /* malformed modules — skip */ }
  }
  c.set('lumiverseModules', lumiverseModules);

  // Collect and save images
  const imagePaths = Object.keys(unzipped).filter(
    (p) => p !== 'card.json' && p !== 'lumiverse_modules.json' && IMAGE_EXTENSIONS.test(p),
  );

  const dir = path.resolve(UPLOAD_PATHS.CHARACTER_IMAGES);
  await mkdir(dir, { recursive: true });

  const charxImages: CharxImageEntry[] = [];
  let primaryImagePath: string | undefined;

  for (const archivePath of imagePaths) {
    const bytes = unzipped[archivePath];
    if (!bytes || bytes.length === 0) continue;

    const ext = path.extname(archivePath).toLowerCase() || '.png';
    const extKey = ext.slice(1);
    const mimeType = MIME_MAP[extKey] || 'image/png';
    const filename = `${crypto.randomUUID()}${ext}`;
    const filePath = path.join(dir, filename);
    const relativePath = `${UPLOAD_PATHS.CHARACTER_IMAGES}/${filename}`;

    await Bun.write(filePath, bytes);

    // Classify by archive path
    const basename = path.basename(archivePath, ext);
    let type: CharxImageEntry['type'];
    let label: string | null = null;

    if (archivePath.startsWith('assets/icon/image/') && basename.startsWith('main')) {
      type = 'avatar';
      primaryImagePath = relativePath;
    } else if (archivePath.startsWith('assets/icon/image/')) {
      type = 'avatar_alt';
      // Try to find label from lumiverse_modules
      if (lumiverseModules?.alternate_avatars) {
        const match = lumiverseModules.alternate_avatars.find((a) => a.path === archivePath);
        label = match?.label || basename;
      } else {
        label = basename;
      }
    } else if (archivePath.startsWith('assets/other/image/expr_')) {
      type = 'expression';
      // Extract label: remove "expr_" prefix
      label = basename.replace(/^expr_/, '');
    } else if (archivePath.startsWith('assets/other/image/gallery_')) {
      type = 'gallery';
    } else if (!primaryImagePath && archivePath.match(/^assets\/(icon|other)\/image\//)) {
      // Fallback: first image in assets as avatar if no main found
      type = 'avatar';
      primaryImagePath = relativePath;
    } else {
      type = 'gallery';
    }

    charxImages.push({
      type,
      label,
      filePath: relativePath,
      mimeType,
      fileSize: bytes.length,
    });
  }

  // If no primary avatar found in assets, check root-level images
  if (!primaryImagePath) {
    const rootImage = imagePaths.find((p) => !p.includes('/'));
    if (rootImage) {
      const entry = charxImages.find((e) => e.filePath.endsWith(path.extname(rootImage)));
      if (entry) {
        entry.type = 'avatar';
        primaryImagePath = entry.filePath;
      }
    }
  }

  c.set('imagePath', primaryImagePath);
  c.set('charxImages', charxImages);

  logger.info(`Extracted .charx: ${charxImages.length} images (primary: ${primaryImagePath || 'none'})`);

  await next();
};
