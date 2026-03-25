import { z } from 'zod';

/** Validates required environment variables at startup. */
const envSchema = z.object({
  DATABASE_URL: z.string().default('postgres://postgres:postgres@localhost:5432/lumihub'),
  PORT: z.string().default('3000'),
  NODE_ENV: z.enum(['development', 'production', 'test']).default('development'),
  UPLOADS_DIR: z.string().default('./uploads'),
  MAX_FILE_SIZE: z.string().default('10485760'),
  MAX_IMAGE_SIZE: z.string().default('5242880'),
  NSFW_THRESHOLD: z.string().default('0.6'),
  DISCORD_CLIENT_ID: z.string().default('1485359841684885666'),
  DISCORD_SECRET_ID: z.string(),
  DISCORD_REDIRECT_URL: z.string().default('http://localhost:5173/api/v1/auth/discord/callback'),
  DISCORD_AUTH_URL: z.string().default('https://discord.com/oauth2/authorize?'),
  JWT_SECRET: z.string().default('super-secret-development-key-change-in-production'),
  FRONTEND_URL: z.string().default('http://localhost:5173'),
  LUMIHUB_PUBLIC_URL: z.string().optional(),
  MODERATOR_DISCORD_IDS: z.string().optional(),
});

const parsed = envSchema.safeParse(process.env);

if (!parsed.success) {
  console.error('Invalid environment variables:', parsed.error.flatten().fieldErrors);
  process.exit(1);
}

if (parsed.data.NODE_ENV === 'production' && parsed.data.JWT_SECRET === 'super-secret-development-key-change-in-production') {
  console.error('FATAL: Default JWT_SECRET must not be used in production. Set a unique JWT_SECRET.');
  process.exit(1);
}

export const env = {
  DATABASE_URL: parsed.data.DATABASE_URL,
  PORT: parseInt(parsed.data.PORT, 10),
  NODE_ENV: parsed.data.NODE_ENV,
  UPLOADS_DIR: parsed.data.UPLOADS_DIR,
  MAX_FILE_SIZE: parseInt(parsed.data.MAX_FILE_SIZE, 10),
  MAX_IMAGE_SIZE: parseInt(parsed.data.MAX_IMAGE_SIZE, 10),
  NSFW_THRESHOLD: parseFloat(parsed.data.NSFW_THRESHOLD),
  DISCORD_CLIENT_ID: parsed.data.DISCORD_CLIENT_ID,
  DISCORD_SECRET_ID: parsed.data.DISCORD_SECRET_ID,
  DISCORD_REDIRECT_URL: parsed.data.DISCORD_REDIRECT_URL,
  DISCORD_AUTH_URL: parsed.data.DISCORD_AUTH_URL,
  JWT_SECRET: parsed.data.JWT_SECRET,
  FRONTEND_URL: parsed.data.FRONTEND_URL,
  LUMIHUB_PUBLIC_URL: parsed.data.LUMIHUB_PUBLIC_URL || parsed.data.FRONTEND_URL,
  MODERATOR_DISCORD_IDS: parsed.data.MODERATOR_DISCORD_IDS
    ? parsed.data.MODERATOR_DISCORD_IDS.split(',').map(id => id.trim()).filter(Boolean)
    : [],
};
