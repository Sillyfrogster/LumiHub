import { AppDataSource } from '../db/connection.ts';

export type LeaderboardType = 'characters' | 'worldbooks' | 'creators';
export type LeaderboardMetric = 'downloads' | 'favorites';
export type LeaderboardPeriod = 'week' | 'month' | 'all';

export interface LeaderboardParams {
  type: LeaderboardType;
  metric: LeaderboardMetric;
  period: LeaderboardPeriod;
  limit?: number;
}

/** Returns the SQL WHERE clause fragment that filters rows to the given period. */
function periodWhere(alias: string, period: LeaderboardPeriod): string {
  if (period === 'week') {
    return `${alias}.updated_at >= NOW() - INTERVAL '7 days'`;
  }
  if (period === 'month') {
    return `${alias}.updated_at >= NOW() - INTERVAL '30 days'`;
  }
  return '1=1';
}

export interface AssetLeaderboardEntry {
  id: string;
  name: string;
  creator: string;
  creatorUsername: string | null;
  creatorDiscordId: string | null;
  avatarUrl: string | null;
  downloads: number;
  favorites: number;
  views: number;
  createdAt: string;
}

export interface CreatorLeaderboardEntry {
  userId: string;
  username: string;
  displayName: string | null;
  avatar: string | null;
  totalDownloads: number;
  totalFavorites: number;
  assetCount: number;
}

/** Returns the top N characters or worldbooks ordered by the requested metric. */
export async function getAssetLeaderboard(
  params: LeaderboardParams,
): Promise<AssetLeaderboardEntry[]> {
  const limit = Math.min(Math.max(params.limit ?? 25, 1), 100);
  const table = params.type === 'characters' ? 'characters' : 'worldbooks';
  const periodClause = periodWhere('a', params.period);
  const orderCol = params.metric === 'favorites' ? 'a.favorites' : 'a.downloads';

  const sql = `
    SELECT
      a.id,
      a.name,
      COALESCE(u.username, '') AS "creatorUsername",
      u.discord_id              AS "creatorDiscordId",
      u.display_name            AS "displayName",
      a.image_path              AS "avatarUrl",
      a.downloads,
      a.favorites,
      a.views,
      a.created_at              AS "createdAt"
    FROM ${table} a
    LEFT JOIN users u ON u.id = a.owner_id
    WHERE a.hidden = false
      AND ${periodClause}
    ORDER BY ${orderCol} DESC
    LIMIT $1
  `;

  const rows: Array<Record<string, any>> = await AppDataSource.query(sql, [limit]);

  return rows.map((r) => ({
    id: r.id,
    name: r.name,
    creator: r.creatorUsername || r.displayName || 'Unknown',
    creatorUsername: r.creatorUsername ?? null,
    creatorDiscordId: r.creatorDiscordId ?? null,
    avatarUrl: r.avatarUrl ?? null,
    downloads: Number(r.downloads),
    favorites: Number(r.favorites),
    views: Number(r.views),
    createdAt: r.createdAt,
  }));
}

/** Returns the top N creators by aggregated downloads + favorites across both tables. */
export async function getCreatorLeaderboard(
  params: LeaderboardParams,
): Promise<CreatorLeaderboardEntry[]> {
  const limit = Math.min(Math.max(params.limit ?? 25, 1), 100);
  const periodClause = periodWhere('a', params.period);
  const orderCol = params.metric === 'favorites' ? 'total_favorites' : 'total_downloads';

  const sql = `
    SELECT
      u.id                      AS "userId",
      u.username,
      u.display_name            AS "displayName",
      u.avatar,
      SUM(a.downloads)::int     AS total_downloads,
      SUM(a.favorites)::int     AS total_favorites,
      COUNT(a.id)::int          AS asset_count
    FROM (
      SELECT id, owner_id, downloads, favorites, updated_at FROM characters WHERE hidden = false
      UNION ALL
      SELECT id, owner_id, downloads, favorites, updated_at FROM worldbooks WHERE hidden = false
    ) a
    INNER JOIN users u ON u.id = a.owner_id
    WHERE ${periodClause}
    GROUP BY u.id, u.username, u.display_name, u.avatar
    ORDER BY ${orderCol} DESC
    LIMIT $1
  `;

  const rows: Array<Record<string, any>> = await AppDataSource.query(sql, [limit]);

  return rows.map((r) => ({
    userId: r.userId,
    username: r.username,
    displayName: r.displayName ?? null,
    avatar: r.avatar ?? null,
    totalDownloads: Number(r.total_downloads),
    totalFavorites: Number(r.total_favorites),
    assetCount: Number(r.asset_count),
  }));
}
