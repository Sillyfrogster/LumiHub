import { Hono } from 'hono';
import { requireAuth, type AuthEnv } from '../middleware/requireAuth.middleware.ts';
import * as PresetService from '../services/presets.service.ts';
import { normalizePresetImport } from '../schemas/preset.schema.ts';
import { FILE_SIZE_LIMITS, UPLOAD_PATHS } from '../utils/constants.ts';
import type { ListQueryParams } from '../types/api.ts';
import {
  isIntakeError,
  parseJsonImport,
  readString,
  readTags,
  savePreviewImage,
} from '../assets/asset-intake.ts';

const presets = new Hono<AuthEnv>();

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

presets.get('/tags', async (c) => {
  const tags = await PresetService.listTags(c.req.query('search') || undefined);
  return c.json({ tags });
});

presets.post('/', requireAuth, async (c) => {
  const ownerId = c.get('userId');
  const formData = await c.req.formData().catch(() => null);
  if (!formData) {
    return c.json({ error: 'Bad Request', message: 'Expected multipart/form-data', statusCode: 400 }, 400);
  }

  const normalized = await parsePresetPayload(formData);
  if (isIntakeError(normalized)) return c.json(normalized.error, 400);

  const image = await savePreviewImage(formData.get('image'), UPLOAD_PATHS.PRESETS);
  if (isIntakeError(image)) return c.json(image.error, 400);

  const preset = await PresetService.createPreset({
    ...normalized,
    name: readString(formData, 'name') || undefined,
    description: readString(formData, 'description') ?? undefined,
    tags: readTags(formData),
    imagePath: image.path,
    ownerId,
  });

  return c.json({ id: preset.id, message: 'Preset created successfully' }, 201);
});

presets.get('/:id/export', async (c) => {
  const preset = await PresetService.getPresetById(c.req.param('id'));
  if (!preset) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  if (preset.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }

  return c.json(PresetService.buildExport(preset));
});

presets.post('/:id/view', async (c) => {
  const result = await PresetService.incrementViews(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

presets.post('/:id/download', async (c) => {
  const result = await PresetService.incrementDownloads(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

presets.put('/:id', requireAuth, async (c) => {
  const id = c.req.param('id');
  const userId = c.get('userId');
  const existing = await PresetService.getPresetById(id);
  if (!existing) {
    return c.json({ error: 'Not Found', message: 'Preset not found', statusCode: 404 }, 404);
  }
  if (existing.owner_id !== userId) {
    return c.json({ error: 'Forbidden', message: 'You do not own this preset', statusCode: 403 }, 403);
  }

  const formData = await c.req.formData().catch(() => null);
  if (!formData) {
    return c.json({ error: 'Bad Request', message: 'Expected multipart/form-data', statusCode: 400 }, 400);
  }

  const payload = hasPayload(formData)
    ? await parsePresetPayload(formData)
    : {};
  if (isIntakeError(payload)) return c.json(payload.error, 400);

  const image = await savePreviewImage(formData.get('image'), UPLOAD_PATHS.PRESETS);
  if (isIntakeError(image)) return c.json(image.error, 400);

  const updated = await PresetService.updatePreset(id, {
    ...payload,
    ...(formData.has('name') ? { name: readString(formData, 'name') || '' } : {}),
    ...(formData.has('description') ? { description: readString(formData, 'description') ?? '' } : {}),
    ...(formData.has('tags') ? { tags: readTags(formData) } : {}),
    ...(image.path ? { imagePath: image.path } : {}),
  });

  return c.json({ data: updated, message: 'Preset updated successfully' });
});

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

async function parsePresetPayload(formData: FormData) {
  return parseJsonImport(formData, {
    fileField: 'preset_file',
    dataField: 'preset_data',
    fileLimit: FILE_SIZE_LIMITS.PRESET,
    fileLimitMessage: 'Preset file exceeds 1MB limit',
    missingMessage: 'preset_file or preset_data is required',
    invalidMessage: 'Invalid preset JSON',
    normalize: normalizePresetImport,
  });
}

function hasPayload(formData: FormData): boolean {
  return formData.has('preset_file') || formData.has('preset_data');
}

export default presets;
