import { Hono } from 'hono';
import { requireAuth, type AuthEnv } from '../middleware/requireAuth.middleware.ts';
import * as ThemeService from '../services/themes.service.ts';
import { normalizeThemeImport } from '../schemas/theme.schema.ts';
import { FILE_SIZE_LIMITS, UPLOAD_PATHS } from '../utils/constants.ts';
import type { ListQueryParams } from '../types/api.ts';
import {
  isIntakeError,
  parseJsonImport,
  readString,
  readTags,
  savePreviewImage,
} from '../assets/asset-intake.ts';

const themes = new Hono<AuthEnv>();

themes.get('/', async (c) => {
  const query: ListQueryParams = {
    page: Number(c.req.query('page')) || undefined,
    limit: Number(c.req.query('limit')) || undefined,
    sort: c.req.query('sort') || undefined,
    order: (c.req.query('order') as 'asc' | 'desc') || undefined,
    search: c.req.query('search') || undefined,
    tags: c.req.query('tags') || undefined,
    ownerId: c.req.query('ownerId') || undefined,
  };

  const result = await ThemeService.listThemes(query);
  return c.json(result);
});

themes.get('/tags', async (c) => {
  const tags = await ThemeService.listTags(c.req.query('search') || undefined);
  return c.json({ tags });
});

themes.post('/', requireAuth, async (c) => {
  const ownerId = c.get('userId');
  const formData = await c.req.formData().catch(() => null);
  if (!formData) {
    return c.json({ error: 'Bad Request', message: 'Expected multipart/form-data', statusCode: 400 }, 400);
  }

  const normalized = await parseThemePayload(formData);
  if (isIntakeError(normalized)) return c.json(normalized.error, 400);

  const image = await savePreviewImage(formData.get('image'), UPLOAD_PATHS.THEMES);
  if (isIntakeError(image)) return c.json(image.error, 400);

  const theme = await ThemeService.createTheme({
    ...normalized,
    name: readString(formData, 'name') || undefined,
    description: readString(formData, 'description') ?? undefined,
    tags: readTags(formData),
    imagePath: image.path,
    ownerId,
  });

  return c.json({ id: theme.id, message: 'Theme created successfully' }, 201);
});

themes.get('/:id/export', async (c) => {
  const theme = await ThemeService.getThemeById(c.req.param('id'));
  if (!theme) {
    return c.json({ error: 'Not Found', message: 'Theme not found', statusCode: 404 }, 404);
  }
  if (theme.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }

  return c.json(ThemeService.buildExport(theme));
});

themes.post('/:id/view', async (c) => {
  const result = await ThemeService.incrementViews(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Theme not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

themes.post('/:id/download', async (c) => {
  const result = await ThemeService.incrementDownloads(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Theme not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

themes.put('/:id', requireAuth, async (c) => {
  const id = c.req.param('id');
  const userId = c.get('userId');
  const existing = await ThemeService.getThemeById(id);
  if (!existing) {
    return c.json({ error: 'Not Found', message: 'Theme not found', statusCode: 404 }, 404);
  }
  if (existing.owner_id !== userId) {
    return c.json({ error: 'Forbidden', message: 'You do not own this theme', statusCode: 403 }, 403);
  }

  const formData = await c.req.formData().catch(() => null);
  if (!formData) {
    return c.json({ error: 'Bad Request', message: 'Expected multipart/form-data', statusCode: 400 }, 400);
  }

  const payload = hasPayload(formData)
    ? await parseThemePayload(formData)
    : {};
  if (isIntakeError(payload)) return c.json(payload.error, 400);

  const image = await savePreviewImage(formData.get('image'), UPLOAD_PATHS.THEMES);
  if (isIntakeError(image)) return c.json(image.error, 400);

  const updated = await ThemeService.updateTheme(id, {
    ...payload,
    ...(formData.has('name') ? { name: readString(formData, 'name') || '' } : {}),
    ...(formData.has('description') ? { description: readString(formData, 'description') ?? '' } : {}),
    ...(formData.has('tags') ? { tags: readTags(formData) } : {}),
    ...(image.path ? { imagePath: image.path } : {}),
  });

  return c.json({ data: updated, message: 'Theme updated successfully' });
});

themes.delete('/:id', requireAuth, async (c) => {
  const id = c.req.param('id');
  const userId = c.get('userId');
  const existing = await ThemeService.getThemeById(id);
  if (!existing) {
    return c.json({ error: 'Not Found', message: 'Theme not found', statusCode: 404 }, 404);
  }
  if (existing.owner_id !== userId) {
    return c.json({ error: 'Forbidden', message: 'You do not own this theme', statusCode: 403 }, 403);
  }

  await ThemeService.deleteTheme(id);
  return c.json({ message: 'Theme deleted successfully' });
});

themes.get('/:id', async (c) => {
  const theme = await ThemeService.getThemeById(c.req.param('id'));
  if (!theme) {
    return c.json({ error: 'Not Found', message: 'Theme not found', statusCode: 404 }, 404);
  }
  if (theme.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }
  return c.json({ data: theme });
});

async function parseThemePayload(formData: FormData) {
  return parseJsonImport(formData, {
    fileField: 'theme_file',
    dataField: 'theme_data',
    fileLimit: FILE_SIZE_LIMITS.THEME,
    fileLimitMessage: 'Theme file exceeds 2MB limit',
    missingMessage: 'theme_file or theme_data is required',
    invalidMessage: 'Invalid theme JSON',
    normalize: normalizeThemeImport,
  });
}

function hasPayload(formData: FormData): boolean {
  return formData.has('theme_file') || formData.has('theme_data');
}

export default themes;
