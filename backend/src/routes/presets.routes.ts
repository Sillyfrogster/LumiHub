import { Hono } from 'hono';
import { requireAuth, type AuthEnv } from '../middleware/requireAuth.middleware.ts';
import * as PresetService from '../services/presets.service.ts';
import { FILE_SIZE_LIMITS, UPLOAD_PATHS } from '../utils/constants.ts';
import type { ListQueryParams } from '../types/api.ts';
import crypto from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const presets = new Hono<AuthEnv>();

/** List public presets with pagination and filters */
presets.get('/', async (c) => {
  const query: ListQueryParams = {
    page: Number(c.req.query('page')) || undefined,
    limit: Number(c.req.query('limit')) || undefined,
    sort: c.req.query('sort') || undefined,
    order: (c.req.query('order') as 'asc' | 'desc') || undefined,
    search: c.req.query('search') || undefined,
    tags: c.req.query('tags') || undefined,
    ownerId: c.req.query('ownerId') || undefined,
  };
  const result = await PresetService.listPresets(query);
  return c.json(result);
});

/** List distinct tags used across all presets */
presets.get('/tags', async (c) => {
  const search = c.req.query('search') || undefined;
  const tags = await PresetService.listTags(search);
  return c.json({ tags });
});

/** Create preset from JSON upload (requires login) */
presets.post('/', requireAuth, async (c) => {
  const ownerId = c.get('userId');
  const formData = await c.req.formData();

  const jsonFile = formData.get('preset_file') as File | null;
  const inlineData = formData.get('preset_data') as string | null;
  const imageFile = formData.get('image') as File | null;
  const nameOverride = formData.get('name') as string | null;
  const descOverride = formData.get('description') as string | null;
  const tagsRaw = formData.get('tags') as string | null;

  let settings: Record<string, any>;

  if (jsonFile) {
    if (jsonFile.size > FILE_SIZE_LIMITS.PRESET) {
      return c.json({ error: 'Bad Request', message: 'Preset file exceeds 1MB limit', statusCode: 400 }, 400);
    }
    const text = await jsonFile.text();
    try {
      settings = JSON.parse(text);
    } catch {
      return c.json({ error: 'Bad Request', message: 'Invalid JSON file', statusCode: 400 }, 400);
    }
  } else if (inlineData) {
    try {
      settings = JSON.parse(inlineData);
    } catch {
      return c.json({ error: 'Bad Request', message: 'Invalid preset_data JSON', statusCode: 400 }, 400);
    }
  } else {
    return c.json({ error: 'Bad Request', message: 'preset_file or preset_data is required', statusCode: 400 }, 400);
  }

  if (!settings || typeof settings !== 'object' || Array.isArray(settings)) {
    return c.json({ error: 'Bad Request', message: 'Preset settings must be a JSON object', statusCode: 400 }, 400);
  }

  const name = (nameOverride || (jsonFile?.name?.replace(/\.json$/i, '') ?? '')).trim();
  if (!name) {
    return c.json({ error: 'Bad Request', message: 'name is required', statusCode: 400 }, 400);
  }

  // Process optional image
  let imagePath: string | undefined;
  if (imageFile && imageFile.size > 0) {
    if (imageFile.size > FILE_SIZE_LIMITS.IMAGE) {
      return c.json({ error: 'Bad Request', message: 'Image exceeds 5MB limit', statusCode: 400 }, 400);
    }
    const ext = imageFile.name?.split('.').pop()?.toLowerCase() || 'png';
    const filename = `${crypto.randomUUID()}.${ext}`;
    const dir = UPLOAD_PATHS.PRESETS;
    await mkdir(dir, { recursive: true });
    const buf = Buffer.from(await imageFile.arrayBuffer());
    const fullPath = path.join(dir, filename);
    await writeFile(fullPath, buf);
    imagePath = fullPath;
  }

  const tags = tagsRaw ? tagsRaw.split(',').map((t) => t.trim()).filter(Boolean) : [];

  const preset = await PresetService.createPreset(
    { name, description: descOverride || '', tags, settings },
    imagePath,
    ownerId,
  );

  return c.json({ id: preset.id, message: 'Preset created successfully' }, 201);
});

/** Export preset settings as JSON */
presets.get('/:id/export', async (c) => {
  const preset = await PresetService.getPresetById(c.req.param('id'));
  if (!preset) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  if (preset.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }

  const filename = preset.name.replace(/[^a-zA-Z0-9_\-. ]/g, '_').replace(/\s+/g, '_').trim() || 'preset';
  return new Response(JSON.stringify(preset.settings, null, 2), {
    headers: {
      'Content-Type': 'application/json',
      'Content-Disposition': `attachment; filename="${filename}.json"`,
    },
  });
});

/** Increment download counter */
presets.post('/:id/download', async (c) => {
  const result = await PresetService.incrementDownloads(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

/** Increment view counter */
presets.post('/:id/view', async (c) => {
  const result = await PresetService.incrementViews(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

/** Delete preset (requires login + ownership) */
presets.delete('/:id', requireAuth, async (c) => {
  const id = c.req.param('id');
  const userId = c.get('userId');

  const existing = await PresetService.getPresetById(id);
  if (!existing) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  if (existing.owner_id !== userId) {
    return c.json({ error: 'Forbidden', message: 'You do not own this preset', statusCode: 403 }, 403);
  }

  await PresetService.deletePreset(id);
  return c.json({ message: 'Preset deleted successfully' });
});

/** Get full preset details — MUST be last /:id route */
presets.get('/:id', async (c) => {
  const preset = await PresetService.getPresetById(c.req.param('id'));
  if (!preset) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  if (preset.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }
  return c.json({ data: preset });
});

export default presets;
