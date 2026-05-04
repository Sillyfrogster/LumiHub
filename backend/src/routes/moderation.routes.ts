import { Hono } from 'hono';
import { requireModerator } from '../middleware/requireModerator.middleware.ts';
import type { AuthEnv } from '../middleware/requireAuth.middleware.ts';
import { AppDataSource } from '../db/connection.ts';
import { logger } from '../utils/logger.ts';
import {
  getHubAssetByRouteSlug,
  HUB_ASSET_ROUTE_SLUGS,
  isHubAssetRouteSlug,
  type HubAssetRouteSlug,
} from '../assets/asset-registry.ts';

const moderation = new Hono<AuthEnv>();
moderation.use('*', requireModerator);

function getRepo(type: HubAssetRouteSlug) {
  return AppDataSource.getRepository(getHubAssetByRouteSlug(type).entity);
}

/** List all hidden content across all types */
moderation.get('/hidden', async (c) => {
  const results: Record<string, any[]> = {};

  for (const type of HUB_ASSET_ROUTE_SLUGS) {
    const items = await getRepo(type).find({
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
  const type = c.req.param('type');
  if (!isHubAssetRouteSlug(type)) {
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
  const type = c.req.param('type');
  if (!isHubAssetRouteSlug(type)) {
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
  const type = c.req.param('type');
  if (!isHubAssetRouteSlug(type)) {
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
