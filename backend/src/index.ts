import 'reflect-metadata';
import { connectDB } from './db/connection.ts';
import { initNSFWModel } from './services/nsfw.service.ts';
import { env } from './env.ts';
import { logger } from './utils/logger.ts';
import app, { websocket } from './app.ts';
import { instanceManager } from './ws/instance-connections.ts';

/** Connects to the database, initializes services, and starts the HTTP server. */
async function start() {
  try {
    logger.info('Starting LumiHub backend...');

    await connectDB();
    await initNSFWModel();

    const server = Bun.serve({
      port: env.PORT,
      hostname: '0.0.0.0',
      fetch: app.fetch,
      websocket,
    });

    instanceManager.startHeartbeat();

    logger.info(`Server running on http://localhost:${server.port}`);
    logger.info(`Environment: ${env.NODE_ENV}`);
  } catch (error) {
    console.error('Failed to start server:', error);
    if (error instanceof Error) console.error(error.stack);
    process.exit(1);
  }
}

start();
