import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { serveStatic } from 'hono/bun';
import { upgradeWebSocket, websocket } from 'hono/bun';
import { errorHandler, notFound } from './middleware/error.middleware.ts';
import charactersRoutes from './routes/characters.routes.ts';
import worldbooksRoutes from './routes/worldbooks.routes.ts';
import authRoutes from './routes/auth.routes.ts';
import userRoutes from './routes/user.routes.ts';
import linkRoutes from './routes/link.routes.ts';
import moderationRoutes from './routes/moderation.routes.ts';
import profileAssetsRoutes from './routes/profile-assets.routes.ts';
import mediaRoutes, { uploadCacheControl } from './routes/media.routes.ts';
import leaderboardRoutes from './routes/leaderboard.routes.ts';
import favoritesRoutes from './routes/favorites.routes.ts';
import { logger } from './utils/logger.ts';
import { env } from './env.ts';
import { opengraphMiddleware, staticPageOgMiddleware } from './middleware/opengraph.middleware.ts';
import { validateLinkToken, updateLastSeen } from './services/link.service.ts';
import { instanceManager } from './ws/instance-connections.ts';

const app = new Hono();

app.use('*', cors({
  origin: env.FRONTEND_URL,
  allowMethods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
  allowHeaders: ['Content-Type', 'Authorization'],
  credentials: true,
}));

app.use('*', async (c, next) => {
  const start = Date.now();
  await next();
  const duration = Date.now() - start;
  logger.info(`${c.req.method} ${c.req.path} - ${c.res.status} (${duration}ms)`);
});

app.use('/uploads/*', async (c, next) => {
  await next();
  if (c.res.ok) {
    c.res.headers.set('Cache-Control', uploadCacheControl);
  }
});
app.use('/uploads/*', serveStatic({ root: './' }));

// Serve built frontend in production
if (env.NODE_ENV === 'production') {
  app.use('/*', serveStatic({ root: '../dist' }));
}

app.use('*', errorHandler);
app.route('/api/v1/characters', charactersRoutes);
app.route('/api/v1/worldbooks', worldbooksRoutes);
app.route('/api/v1/auth', authRoutes);
app.route('/api/v1/media', mediaRoutes);
app.route('/api/v1/users/me/assets', profileAssetsRoutes);
app.route('/api/v1/user', userRoutes);
app.route('/api/v1/users', userRoutes);
app.route('/api/v1/link', linkRoutes);
app.route('/api/v1/links', linkRoutes);
app.route('/api/v1/leaderboard', leaderboardRoutes);
app.route('/api/v1/favorites', favoritesRoutes);
app.route('/api/v1/moderation', moderationRoutes);
app.route('/api/v1/profile-assets', profileAssetsRoutes);

// WebSocket endpoint for Lumiverse instance connections
app.get('/api/v1/ws/instance', upgradeWebSocket((c) => {
  let instanceId: string | null = null;
  let userId: string | null = null;

  return {
    async onOpen(_event, ws) {
      const url = new URL(c.req.url);
      const token = url.searchParams.get('token');
      if (!token) {
        ws.close(1008, 'Token required');
        return;
      }

      const instance = await validateLinkToken(token);
      if (!instance) {
        ws.close(1008, 'Invalid or revoked token');
        return;
      }

      instanceId = instance.id;
      userId = instance.user_id;

      const raw = (ws as any).raw as import('bun').ServerWebSocket<unknown>;
      if (raw) {
        instanceManager.register(instanceId, userId, raw);
      }
      await updateLastSeen(instanceId);

      ws.send(JSON.stringify({
        type: 'auth_ok',
        id: crypto.randomUUID(),
        payload: { instanceId, userId },
        timestamp: Date.now(),
      }));

      logger.info(`[WS] Instance ${instance.instance_name} (${instanceId}) connected`);
    },
    onMessage(event, _ws) {
      if (!instanceId) return;
      const raw = (_ws as any).raw as import('bun').ServerWebSocket<unknown>;
      if (raw) {
        instanceManager.handleMessage(raw, event.data as string);
      }
    },
    onClose(_event, _ws) {
      if (instanceId) {
        logger.info(`[WS] Instance ${instanceId} disconnected`);
      }
      const raw = (_ws as any).raw as import('bun').ServerWebSocket<unknown>;
      if (raw) {
        instanceManager.unregister(raw);
      }
    },
  };
}));

// OpenGraph meta injection (must be before SPA fallback)
if (env.NODE_ENV === 'production') {
  // Static pages (homepage, browse pages)
  app.get('/', staticPageOgMiddleware);
  app.get('/characters', staticPageOgMiddleware);
  app.get('/worldbooks', staticPageOgMiddleware);
  app.get('/themes', staticPageOgMiddleware);
  app.get('/presets', staticPageOgMiddleware);
  app.get('/leaderboard', staticPageOgMiddleware);
  // Dynamic content pages
  app.get('/characters/:id', opengraphMiddleware);
  app.get('/characters/:creator/:name', opengraphMiddleware); // Chub cards: decoded /Creator/name
  app.get('/worldbooks/:id', opengraphMiddleware);
  app.get('/presets/:id', opengraphMiddleware);
  app.get('/themes/:id', opengraphMiddleware);
}

// SPA fallback — serve index.html for non-API routes in production
if (env.NODE_ENV === 'production') {
  app.get('*', serveStatic({ root: '../dist', path: '/index.html' }));
}

app.notFound(notFound);

export default app;
export { websocket };
