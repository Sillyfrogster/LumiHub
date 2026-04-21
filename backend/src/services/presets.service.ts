import { unlink } from 'node:fs/promises';
import path from 'node:path';
import { AppDataSource } from '../db/connection.ts';
import { Preset } from '../entities/Preset.entity.ts';
import { logger } from '../utils/logger.ts';
import { UPLOAD_PATHS } from '../utils/constants.ts';
import type { ListQueryParams } from '../types/api.ts';

const repo = () => AppDataSource.getRepository(Preset);

export async function listPresets(params: ListQueryParams) {
  const page = Math.max(params.page ?? 1, 1);
  const limit = Math.min(Math.max(params.limit ?? 20, 1), 100);
  const skip = (page - 1) * limit;
  const ALLOWED_SORT_FIELDS = ['created_at', 'updated_at', 'downloads', 'views', 'name'];
  const sortField = ALLOWED_SORT_FIELDS.includes(params.sort ?? '') ? params.sort! : 'created_at';
  const sortOrder = (params.order === 'asc' ? 'ASC' : 'DESC') as 'ASC' | 'DESC';

  const qb = repo().createQueryBuilder('preset')
    .leftJoinAndSelect('preset.owner', 'owner')
    .where('preset.hidden = false')
    .orderBy(`preset.${sortField}`, sortOrder)
    .skip(skip)
    .take(limit);

  if (params.search) {
    qb.andWhere('preset.name ILIKE :search', { search: `%${params.search}%` });
  }

  if (params.ownerId) {
    qb.andWhere('preset.owner_id = :ownerId', { ownerId: params.ownerId });
  }

  if (params.tags) {
    const tagArray = params.tags.split(',').map(t => t.trim().toLowerCase()).filter(Boolean);
    if (tagArray.length > 0) {
      qb.andWhere(
        `EXISTS (SELECT 1 FROM jsonb_array_elements_text(preset.tags) AS t WHERE LOWER(t) IN (:...tagValues))`,
        { tagValues: tagArray },
      );
    }
  }

  const [presets, total] = await qb.getManyAndCount();

  return {
    data: presets,
    pagination: { page, limit, total, totalPages: Math.ceil(total / limit) },
  };
}

/** Returns distinct tags used across all presets, with usage counts. */
export async function listTags(search?: string) {
  const params: unknown[] = [];
  let where = '';

  if (search) {
    where = 'WHERE tag ILIKE $1';
    params.push(`%${search}%`);
  }

  const hiddenClause = where ? 'AND hidden = false' : 'WHERE hidden = false';
  const sql = `
    SELECT tag AS name, COUNT(*)::int AS count
    FROM presets, jsonb_array_elements_text(tags) AS tag
    ${where} ${hiddenClause}
    GROUP BY tag
    ORDER BY count DESC
    LIMIT 200
  `;

  const rows: { name: string; count: number }[] =
    await AppDataSource.query(sql, params);
  return rows;
}

export async function getPresetById(id: string) {
  return repo().findOne({ where: { id }, relations: ['owner'] });
}

export interface PresetCreateData {
  name: string;
  description?: string;
  tags?: string[];
  settings: Record<string, any>;
}

export async function createPreset(
  data: PresetCreateData,
  imagePath: string | undefined,
  ownerId?: string | null,
) {
  const preset = repo().create({
    name: data.name,
    description: data.description ?? '',
    tags: data.tags ?? [],
    settings: data.settings,
    image_path: imagePath ?? null,
    owner_id: ownerId ?? null,
  });
  return repo().save(preset);
}

export async function deletePreset(id: string) {
  const existing = await repo().findOneBy({ id });
  if (!existing) return false;

  if (existing.image_path) {
    try {
      const absolute = resolveUploadPath(existing.image_path);
      if (!absolute) {
        logger.warn(`Refusing to delete path outside uploads root: ${existing.image_path}`);
        await repo().remove(existing);
        return true;
      }
      await unlink(absolute);
      logger.info(`Deleted preset image: ${absolute}`);
    } catch (err) {
      logger.warn(`Could not delete preset image: ${existing.image_path}`, err);
    }
  }

  await repo().remove(existing);
  return true;
}

export async function incrementDownloads(id: string) {
  const result = await repo()
    .createQueryBuilder()
    .update(Preset)
    .set({ downloads: () => 'downloads + 1' })
    .where('id = :id', { id })
    .returning('downloads')
    .execute();

  if (result.affected === 0) return null;

  return { downloads: result.raw[0]?.downloads as number };
}

export async function incrementViews(id: string) {
  const result = await repo()
    .createQueryBuilder()
    .update(Preset)
    .set({ views: () => 'views + 1' })
    .where('id = :id', { id })
    .returning('views')
    .execute();

  if (result.affected === 0) return null;

  return { views: result.raw[0]?.views as number };
}

function resolveUploadPath(relativePath: string): string | null {
  const uploadsRoot = path.resolve(UPLOAD_PATHS.ROOT);
  // Strip leading slashes and the "uploads/" prefix if present to get a relative sub-path
  const normalized = relativePath.replace(/^\/+/, '');
  const absolute = path.resolve(uploadsRoot, normalized.startsWith(`${UPLOAD_PATHS.ROOT}/`)
    ? normalized.slice(UPLOAD_PATHS.ROOT.length + 1)
    : normalized);

  // Reject any path that escapes the uploads root to prevent path-traversal attacks
  if (absolute === uploadsRoot || absolute.startsWith(`${uploadsRoot}${path.sep}`)) {
    return absolute;
  }

  return null;
}
