import { unlink } from 'node:fs/promises';
import path from 'node:path';
import { AppDataSource } from '../db/connection.ts';
import { Worldbook } from '../entities/Worldbook.entity.ts';
import { logger } from '../utils/logger.ts';
import { UPLOAD_PATHS } from '../utils/constants.ts';
import type { ListQueryParams } from '../types/api.ts';

const repo = () => AppDataSource.getRepository(Worldbook);

export async function listWorldbooks(params: ListQueryParams) {
  const page = Math.max(params.page ?? 1, 1);
  const limit = Math.min(Math.max(params.limit ?? 20, 1), 100);
  const skip = (page - 1) * limit;
  const ALLOWED_SORT_FIELDS = ['created_at', 'updated_at', 'downloads', 'name'];
  const sortField = ALLOWED_SORT_FIELDS.includes(params.sort ?? '') ? params.sort! : 'created_at';
  const sortOrder = (params.order === 'asc' ? 'ASC' : 'DESC') as 'ASC' | 'DESC';

  const qb = repo().createQueryBuilder('worldbook')
    .leftJoinAndSelect('worldbook.owner', 'owner')
    .where('worldbook.hidden = false')
    .orderBy(`worldbook.${sortField}`, sortOrder)
    .skip(skip)
    .take(limit);

  if (params.search) {
    qb.andWhere('worldbook.name ILIKE :search', { search: `%${params.search}%` });
  }

  if (params.ownerId) {
    qb.andWhere('worldbook.owner_id = :ownerId', { ownerId: params.ownerId });
  }

  if (params.tags) {
    const tagArray = params.tags.split(',').map(t => t.trim().toLowerCase()).filter(Boolean);
    if (tagArray.length > 0) {
      qb.andWhere(
        `EXISTS (SELECT 1 FROM jsonb_array_elements_text(worldbook.tags) AS t WHERE LOWER(t) IN (:...tagValues))`,
        { tagValues: tagArray },
      );
    }
  }

  const [worldbooks, total] = await qb.getManyAndCount();

  return {
    data: worldbooks,
    pagination: { page, limit, total, totalPages: Math.ceil(total / limit) },
  };
}

/** Returns distinct tags used across all worldbooks, with usage counts. */
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
    FROM worldbooks, jsonb_array_elements_text(tags) AS tag
    ${where} ${hiddenClause}
    GROUP BY tag
    ORDER BY count DESC
    LIMIT 200
  `;

  const rows: { name: string; count: number }[] =
    await AppDataSource.query(sql, params);
  return rows;
}

export async function getWorldbookById(id: string) {
  return repo().findOne({ where: { id }, relations: ['owner'] });
}

export interface WorldbookCreateData {
  name: string;
  description?: string;
  tags?: string[];
  creator?: string;
  entries: Record<string, any>[];
}

export async function createWorldbook(
  data: WorldbookCreateData,
  imagePath: string | undefined,
  ownerId?: string | null,
) {
  const worldbook = repo().create({
    name: data.name,
    description: data.description ?? '',
    tags: data.tags ?? [],
    entries: data.entries,
    image_path: imagePath ?? null,
    owner_id: ownerId ?? null,
  });
  return repo().save(worldbook);
}

export async function deleteWorldbook(id: string) {
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
      logger.info(`Deleted worldbook image: ${absolute}`);
    } catch (err) {
      logger.warn(`Could not delete worldbook image: ${existing.image_path}`, err);
    }
  }

  await repo().remove(existing);
  return true;
}

export async function incrementDownloads(id: string) {
  const wb = await repo().findOneBy({ id });
  if (!wb) return null;
  wb.downloads += 1;
  await repo().save(wb);
  return { downloads: wb.downloads };
}

export async function incrementViews(id: string) {
  const result = await repo()
    .createQueryBuilder()
    .update(Worldbook)
    .set({ views: () => 'views + 1' })
    .where('id = :id', { id })
    .returning('views')
    .execute();

  if (result.affected === 0) return null;

  return { views: result.raw[0]?.views as number };
}

/**
 * Normalizes entries from multiple lorebook formats into a standard array.
 * Accepts: Lumiverse export, CCSv3 character_book, SillyTavern lorebooks.
 */
export function normalizeLorebookEntries(raw: unknown): Record<string, any>[] {
  if (Array.isArray(raw)) return raw;

  // Object-keyed entries (CCSv3/ST format: { "0": {...}, "1": {...} })
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
    return Object.values(raw as Record<string, any>);
  }

  return [];
}

/**
 * Auto-detects the lorebook format and extracts entries + metadata.
 */
export function parseLorebookFile(json: Record<string, any>): {
  name: string;
  description: string;
  entries: Record<string, any>[];
} {
  // Lumiverse format: { type: "world_book", entries: [...] }
  if (json.type === 'world_book' && json.entries) {
    return {
      name: json.name || 'Imported Worldbook',
      description: json.description || '',
      entries: normalizeLorebookEntries(json.entries),
    };
  }

  // CCSv3 character_book wrapper: { entries: {...} or [...] }
  if (json.entries) {
    return {
      name: json.name || 'Imported Worldbook',
      description: json.description || '',
      entries: normalizeLorebookEntries(json.entries),
    };
  }

  // Bare array of entries
  if (Array.isArray(json)) {
    return {
      name: 'Imported Worldbook',
      description: '',
      entries: json,
    };
  }

  return { name: 'Imported Worldbook', description: '', entries: [] };
}

function resolveUploadPath(relativePath: string): string | null {
  const uploadsRoot = path.resolve(UPLOAD_PATHS.ROOT);
  const normalized = relativePath.replace(/^\/+/, '');
  const absolute = path.resolve(uploadsRoot, normalized.startsWith(`${UPLOAD_PATHS.ROOT}/`)
    ? normalized.slice(UPLOAD_PATHS.ROOT.length + 1)
    : normalized);

  if (absolute === uploadsRoot || absolute.startsWith(`${uploadsRoot}${path.sep}`)) {
    return absolute;
  }

  return null;
}
