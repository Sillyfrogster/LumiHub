import { Hono } from 'hono';
import { requireAuth, type AuthEnv } from '../middleware/requireAuth.middleware.ts';
import { AppDataSource } from '../db/connection.ts';
import { Favorite } from '../entities/Favorite.entity.ts';
import { getHubAsset, isHubAssetType, type HubAssetType } from '../assets/asset-registry.ts';

const favorites = new Hono<AuthEnv>();

const favRepo = () => AppDataSource.getRepository(Favorite);

/**
 * POST /api/v1/favorites/toggle
 * Body: { assetType: 'character'|'worldbook'|'theme'|'preset', assetId: '<uuid>' }
 * Toggles the favorite — adds it if missing, removes it if present.
 * Also atomically updates the favorites counter on the asset row.
 * Returns { favorited: boolean, favorites: number }
 */
favorites.post('/toggle', requireAuth, async (c) => {
  const userId = c.get('userId');
  const body = await c.req.json().catch(() => null);

  if (!body || !body.assetType || !body.assetId) {
    return c.json({ error: 'Bad Request', message: 'assetType and assetId are required', statusCode: 400 }, 400);
  }

  const assetType: HubAssetType = body.assetType;
  const assetId: string = body.assetId;

  if (!isHubAssetType(assetType)) {
    return c.json({ error: 'Bad Request', message: 'assetType must be a valid asset type', statusCode: 400 }, 400);
  }

  const assetRepo = AppDataSource.getRepository(getHubAsset(assetType).entity);

  const asset = await assetRepo.findOneBy({ id: assetId });
  if (!asset) {
    return c.json({ error: 'Not Found', message: 'Asset not found', statusCode: 404 }, 404);
  }

  const existing = await favRepo().findOneBy({ user_id: userId, asset_type: assetType, asset_id: assetId });

  if (existing) {
    await favRepo().remove(existing);
    await assetRepo.createQueryBuilder()
      .update()
      .set({ favorites: () => 'GREATEST(favorites - 1, 0)' })
      .where('id = :id', { id: assetId })
      .execute();

    const updated = await assetRepo.findOneBy({ id: assetId });
    return c.json({ favorited: false, favorites: updated?.favorites ?? 0 });
  } else {
    const fav = favRepo().create({ user_id: userId, asset_type: assetType, asset_id: assetId });
    await favRepo().save(fav);
    await assetRepo.createQueryBuilder()
      .update()
      .set({ favorites: () => 'favorites + 1' })
      .where('id = :id', { id: assetId })
      .execute();

    const updated = await assetRepo.findOneBy({ id: assetId });
    return c.json({ favorited: true, favorites: updated?.favorites ?? 0 });
  }
});

/**
 * GET /api/v1/favorites/check?assetType=character&assetId=<uuid>
 * Returns { favorited: boolean } for the currently authenticated user.
 */
favorites.get('/check', requireAuth, async (c) => {
  const userId = c.get('userId');
  const assetType = c.req.query('assetType');
  const assetId = c.req.query('assetId');

  if (!assetType || !assetId || !isHubAssetType(assetType)) {
    return c.json({ error: 'Bad Request', message: 'Valid assetType and assetId query params are required', statusCode: 400 }, 400);
  }

  const existing = await favRepo().findOneBy({ user_id: userId, asset_type: assetType, asset_id: assetId });
  return c.json({ favorited: !!existing });
});

export default favorites;
