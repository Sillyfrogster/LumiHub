import { Hono } from 'hono';
import { requireModerator } from '../middleware/requireModerator.middleware.ts';
import type { AuthEnv } from '../middleware/requireAuth.middleware.ts';
import { AppDataSource } from '../db/connection.ts';
import { Character } from '../entities/Character.entity.ts';
import { Worldbook } from '../entities/Worldbook.entity.ts';
import { Preset } from '../entities/Preset.entity.ts';
import { Theme } from '../entities/Theme.entity.ts';
import { logger } from '../utils/logger.ts';

const moderation = new Hono<AuthEnv>();
moderation.use('*', requireModerator);

const CONTENT_TYPES = {
  characters: Character,
  worldbooks: Worldbook,
  presets: Preset,
  themes: Theme,
} as const;

type ContentType = keyof typeof CONTENT_TYPES;

function getRepo(type: ContentType) {
  return AppDataSource.getRepository(CONTENT_TYPES[type]);
}

/** List all hidden content across all types */
moderation.get('/hidden', async (c) => {
  const results: Record<string, any[]> = {};

  for (const [type, entity] of Object.entries(CONTENT_TYPES)) {
    const items = await AppDataSource.getRepository(entity).find({
      where: { hidden: true },
      order: { updated_at: 'DESC' },
    });
    if (items.length > 0) {
      results[type] = items;
    }
  }

  return c.json({ data: results });
});

/** Hide a piece of content */
moderation.patch('/:type/:id/hide', async (c) => {
  const type = c.req.param('type') as ContentType;
  if (!CONTENT_TYPES[type]) {
    return c.json({ error: 'Bad Request', message: `Invalid content type: ${type}`, statusCode: 400 }, 400);
  }

  const repo = getRepo(type);
  const item = await repo.findOneBy({ id: c.req.param('id') });
  if (!item) {
    return c.json({ error: 'Not Found', message: 'Content not found', statusCode: 404 }, 404);
  }

  item.hidden = true;
  await repo.save(item);

  logger.info(`[Moderation] ${type}/${item.id} hidden by user ${c.get('userId')}`);
  return c.json({ message: 'Content hidden', id: item.id, type });
});

/** Unhide a piece of content */
moderation.patch('/:type/:id/unhide', async (c) => {
  const type = c.req.param('type') as ContentType;
  if (!CONTENT_TYPES[type]) {
    return c.json({ error: 'Bad Request', message: `Invalid content type: ${type}`, statusCode: 400 }, 400);
  }

  const repo = getRepo(type);
  const item = await repo.findOneBy({ id: c.req.param('id') });
  if (!item) {
    return c.json({ error: 'Not Found', message: 'Content not found', statusCode: 404 }, 404);
  }

  item.hidden = false;
  await repo.save(item);

  logger.info(`[Moderation] ${type}/${item.id} unhidden by user ${c.get('userId')}`);
  return c.json({ message: 'Content unhidden', id: item.id, type });
});

/** Delete content (moderator override — bypasses ownership check) */
moderation.delete('/:type/:id', async (c) => {
  const type = c.req.param('type') as ContentType;
  if (!CONTENT_TYPES[type]) {
    return c.json({ error: 'Bad Request', message: `Invalid content type: ${type}`, statusCode: 400 }, 400);
  }

  const repo = getRepo(type);
  const item = await repo.findOneBy({ id: c.req.param('id') });
  if (!item) {
    return c.json({ error: 'Not Found', message: 'Content not found', statusCode: 404 }, 404);
  }

  await repo.remove(item);

  logger.info(`[Moderation] ${type}/${c.req.param('id')} deleted by user ${c.get('userId')}`);
  return c.json({ message: 'Content deleted', id: c.req.param('id'), type });
});

export default moderation;
