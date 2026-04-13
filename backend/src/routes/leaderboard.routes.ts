import { Hono } from 'hono';
import * as LeaderboardService from '../services/leaderboard.service.ts';
import type {
  LeaderboardType,
  LeaderboardMetric,
  LeaderboardPeriod,
} from '../services/leaderboard.service.ts';

const leaderboard = new Hono();

const VALID_TYPES: LeaderboardType[] = ['characters', 'worldbooks', 'creators'];
const VALID_METRICS: LeaderboardMetric[] = ['downloads', 'favorites'];
const VALID_PERIODS: LeaderboardPeriod[] = ['week', 'month', 'all'];

/**
 * GET /api/v1/leaderboard
 *   ?type=characters|worldbooks|creators
 *   &metric=downloads|favorites
 *   &period=week|month|all
 *   &limit=25 (max 100)
 */
leaderboard.get('/', async (c) => {
  const rawType = c.req.query('type') ?? 'characters';
  const rawMetric = c.req.query('metric') ?? 'downloads';
  const rawPeriod = c.req.query('period') ?? 'all';
  const rawLimit = c.req.query('limit');

  if (!VALID_TYPES.includes(rawType as LeaderboardType)) {
    return c.json({ error: 'Bad Request', message: `Invalid type. Must be one of: ${VALID_TYPES.join(', ')}`, statusCode: 400 }, 400);
  }
  if (!VALID_METRICS.includes(rawMetric as LeaderboardMetric)) {
    return c.json({ error: 'Bad Request', message: `Invalid metric. Must be one of: ${VALID_METRICS.join(', ')}`, statusCode: 400 }, 400);
  }
  if (!VALID_PERIODS.includes(rawPeriod as LeaderboardPeriod)) {
    return c.json({ error: 'Bad Request', message: `Invalid period. Must be one of: ${VALID_PERIODS.join(', ')}`, statusCode: 400 }, 400);
  }

  const type = rawType as LeaderboardType;
  const metric = rawMetric as LeaderboardMetric;
  const period = rawPeriod as LeaderboardPeriod;
  const limit = rawLimit ? Number(rawLimit) : undefined;

  const params = { type, metric, period, limit };

  const data =
    type === 'creators'
      ? await LeaderboardService.getCreatorLeaderboard(params)
      : await LeaderboardService.getAssetLeaderboard(params);

  return c.json({ data });
});

export default leaderboard;
