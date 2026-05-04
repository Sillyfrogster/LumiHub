import { Hono } from 'hono';
import { requireAuth, type AuthEnv } from '../middleware/requireAuth.middleware.ts';
import * as WorldbookService from '../services/worldbooks.service.ts';
import { FILE_SIZE_LIMITS, UPLOAD_PATHS } from '../utils/constants.ts';
import type { ListQueryParams } from '../types/api.ts';
import {
  isIntakeError,
  parseJsonImport,
  readString,
  readTags,
  savePreviewImage,
} from '../assets/asset-intake.ts';

const worldbooks = new Hono<AuthEnv>();

/** List public worldbooks with pagination and filters */
worldbooks.get('/', async (c) => {
  const query: ListQueryParams = {
    page: Number(c.req.query('page')) || undefined,
    limit: Number(c.req.query('limit')) || undefined,
    sort: c.req.query('sort') || undefined,
    order: (c.req.query('order') as 'asc' | 'desc') || undefined,
    search: c.req.query('search') || undefined,
    tags: c.req.query('tags') || undefined,
    ownerId: c.req.query('ownerId') || undefined,
  };
  const result = await WorldbookService.listWorldbooks(query);
  return c.json(result);
});

/** List distinct tags used across all worldbooks */
worldbooks.get('/tags', async (c) => {
  const search = c.req.query('search') || undefined;
  const tags = await WorldbookService.listTags(search);
  return c.json({ tags });
});

/** Create worldbook from JSON upload (requires login) */
worldbooks.post('/', requireAuth, async (c) => {
  const ownerId = c.get('userId');
  const formData = await c.req.formData();

  const parsed = await parseJsonImport(formData, {
    fileField: 'worldbook_file',
    dataField: 'worldbook_data',
    fileLimit: FILE_SIZE_LIMITS.WORLDBOOK,
    fileLimitMessage: 'Worldbook file exceeds 5MB limit',
    missingMessage: 'worldbook_file or worldbook_data is required',
    invalidMessage: 'Invalid worldbook JSON',
    normalize: (input) => WorldbookService.parseLorebookFile(input as Record<string, any>),
  });
  if (isIntakeError(parsed)) return c.json(parsed.error, 400);

  if (parsed.entries.length === 0) {
    return c.json({ error: 'Bad Request', message: 'No valid lorebook entries found', statusCode: 400 }, 400);
  }

  const image = await savePreviewImage(formData.get('image'), UPLOAD_PATHS.WORLDBOOKS);
  if (isIntakeError(image)) return c.json(image.error, 400);

  const worldbook = await WorldbookService.createWorldbook(
    {
      name: readString(formData, 'name') || parsed.name,
      description: readString(formData, 'description') || parsed.description,
      tags: readTags(formData),
      entries: parsed.entries,
    },
    image.path ?? undefined,
    ownerId,
  );

  return c.json({ id: worldbook.id, message: 'Worldbook created successfully', entryCount: parsed.entries.length }, 201);
});

/** Export worldbook entries as JSON */
worldbooks.get('/:id/export', async (c) => {
  const wb = await WorldbookService.getWorldbookById(c.req.param('id'));
  if (!wb) {
    return c.json({ error: 'Not Found', message: 'Worldbook not found', statusCode: 404 }, 404);
  }
  if (wb.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }

  const entries = WorldbookService.normalizeLorebookEntries(wb.entries);
  return c.json({
    name: wb.name,
    description: wb.description,
    entries,
  });
});

/** Increment download counter */
worldbooks.post('/:id/download', async (c) => {
  const result = await WorldbookService.incrementDownloads(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Worldbook not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

/** Increment view counter */
worldbooks.post('/:id/view', async (c) => {
  const result = await WorldbookService.incrementViews(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Worldbook not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

/** Delete worldbook (requires login + ownership) */
worldbooks.delete('/:id', requireAuth, async (c) => {
  const id = c.req.param('id');
  const userId = c.get('userId');

  const existing = await WorldbookService.getWorldbookById(id);
  if (!existing) {
    return c.json({ error: 'Not Found', message: 'Worldbook not found', statusCode: 404 }, 404);
  }
  if (existing.owner_id !== userId) {
    return c.json({ error: 'Forbidden', message: 'You do not own this worldbook', statusCode: 403 }, 403);
  }

  await WorldbookService.deleteWorldbook(id);
  return c.json({ message: 'Worldbook deleted successfully' });
});

/** Get full worldbook details — MUST be last /:id route */
worldbooks.get('/:id', async (c) => {
  const wb = await WorldbookService.getWorldbookById(c.req.param('id'));
  if (!wb) {
    return c.json({ error: 'Not Found', message: 'Worldbook not found', statusCode: 404 }, 404);
  }
  if (wb.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }
  return c.json({ data: wb });
});

export default worldbooks;
