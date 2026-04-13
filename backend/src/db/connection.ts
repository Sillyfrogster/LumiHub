import { DataSource } from 'typeorm';
import { env } from '../env.ts';
import { logger } from '../utils/logger.ts';
import { Character } from '../entities/Character.entity.ts';
import { User } from '../entities/User.entity.ts';
import { Worldbook } from '../entities/Worldbook.entity.ts';
import { Preset } from '../entities/Preset.entity.ts';
import { Theme } from '../entities/Theme.entity.ts';
import { LinkedInstance } from '../entities/LinkedInstance.entity.ts';
import { LinkCode } from '../entities/LinkCode.entity.ts';
import { CharacterImage } from '../entities/CharacterImage.entity.ts';
import { ProfileAsset } from '../entities/ProfileAsset.entity.ts';
import { InstallManifest } from '../entities/InstallManifest.entity.ts';
import { Favorite } from '../entities/Favorite.entity.ts';
import { PostgresQueryRunner } from 'typeorm/driver/postgres/PostgresQueryRunner.js';

/** Serializes queries to work around TypeORM issue #12055. */
const originalQuery = PostgresQueryRunner.prototype.query;
PostgresQueryRunner.prototype.query = function (this: any, ...args: any[]) {
  if (!this._queryQueue) {
    this._queryQueue = Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    this._queryQueue = this._queryQueue.then(async () => {
      try {
        // @ts-ignore
        resolve(await originalQuery.apply(this, args));
      } catch (err) {
        reject(err);
      }
    });
  });
};

/**
 * Global TypeORM DataSource configuration.
 */
export const AppDataSource = new DataSource({
  type: 'postgres',
  url: env.DATABASE_URL,
  synchronize: true,
  logging: ['error'],
  entities: [Character, User, Worldbook, Preset, Theme, LinkedInstance, LinkCode, CharacterImage, ProfileAsset, InstallManifest, Favorite],
  extra: { max: 20 },
});

/** Initializes the TypeORM DataSource and connects to PostgreSQL. */
export async function connectDB(): Promise<void> {
  try {
    await AppDataSource.initialize();
    logger.info(`PostgreSQL connected`);
  } catch (error) {
    logger.error('PostgreSQL connection error:', error);
    process.exit(1);
  }
}
