import type { MiddlewareHandler } from 'hono';
import { getCookie } from 'hono/cookie';
import { verify } from 'hono/jwt';
import { AppDataSource } from '../db/connection.ts';
import { User } from '../entities/User.entity.ts';
import { env } from '../env.ts';
import type { AuthEnv } from './requireAuth.middleware.ts';

/**
 * Requires a valid JWT session belonging to a user with 'moderator' role.
 */
export const requireModerator: MiddlewareHandler<AuthEnv> = async (c, next) => {
  const token = getCookie(c, 'lumihub_session');

  if (!token) {
    return c.json({ error: 'Unauthorized', message: 'Authentication required', statusCode: 401 }, 401);
  }

  let userId: string;
  try {
    const payload = await verify(token, env.JWT_SECRET, 'HS256');
    userId = payload.id as string;
    c.set('userId', userId);
  } catch {
    return c.json({ error: 'Unauthorized', message: 'Invalid or expired token', statusCode: 401 }, 401);
  }

  const user = await AppDataSource.getRepository(User).findOneBy({ id: userId });
  if (!user || user.role !== 'moderator') {
    return c.json({ error: 'Forbidden', message: 'Moderator access required', statusCode: 403 }, 403);
  }

  await next();
};
