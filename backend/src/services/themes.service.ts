import { unlink } from 'node:fs/promises';
import path from 'node:path';
import { AppDataSource } from '../db/connection.ts';
import { Theme } from '../entities/Theme.entity.ts';
import { logger } from '../utils/logger.ts';
import { UPLOAD_PATHS } from '../utils/constants.ts';
import type { ListQueryParams } from '../types/api.ts';
import { buildThemeExport, type NormalizedThemeImport } from '../schemas/theme.schema.ts';

const repo = () => AppDataSource.getRepository(Theme);

export interface ThemeCreateData extends NormalizedThemeImport {
  name?: string;
  description?: string;
  tags?: string[];
  imagePath?: string | null;
  ownerId?: string | null;
}

export async function listThemes(params: ListQueryParams) {
  const page = Math.max(params.page ?? 1, 1);
  const limit = Math.min(Math.max(params.limit ?? 20, 1), 100);
  const skip = (page - 1) * limit;
  const allowedSortFields = ['created_at', 'updated_at', 'downloads', 'views', 'favorites', 'name'];
  const sortField = allowedSortFields.includes(params.sort ?? '') ? params.sort! : 'created_at';
  const sortOrder = (params.order === 'asc' ? 'ASC' : 'DESC') as 'ASC' | 'DESC';

  const qb = repo().createQueryBuilder('theme')
    .leftJoinAndSelect('theme.owner', 'owner')
    .where('theme.hidden = false')
    .orderBy(`theme.${sortField}`, sortOrder)
    .skip(skip)
    .take(limit);

  if (params.search) {
    qb.andWhere('(theme.name ILIKE :search OR theme.description ILIKE :search)', { search: `%${params.search}%` });
  }

  if (params.ownerId) {
    qb.andWhere('theme.owner_id = :ownerId', { ownerId: params.ownerId });
  }

  if (params.tags) {
    const tagArray = params.tags.split(',').map((t) => t.trim().toLowerCase()).filter(Boolean);
    if (tagArray.length > 0) {
      qb.andWhere(
        `EXISTS (SELECT 1 FROM jsonb_array_elements_text(theme.tags) AS t WHERE LOWER(t) IN (:...tagValues))`,
        { tagValues: tagArray },
      );
    }
  }

  const [themes, total] = await qb.getManyAndCount();

  return {
    data: themes,
    pagination: { page, limit, total, totalPages: Math.ceil(total / limit) },
  };
}

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
    FROM themes, jsonb_array_elements_text(tags) AS tag
    ${where} ${hiddenClause}
    GROUP BY tag
    ORDER BY count DESC
    LIMIT 200
  `;

  const rows: { name: string; count: number }[] = await AppDataSource.query(sql, params);
  return rows;
}

export async function getThemeById(id: string) {
  return repo().findOne({ where: { id }, relations: ['owner'] });
}

export async function createTheme(data: ThemeCreateData) {
  const theme = repo().create({
    name: data.name || String(data.config.name || 'Untitled Theme'),
    description: data.description ?? '',
    config: data.config,
    tags: data.tags ?? [],
    schema_version: data.schemaVersion,
    compatibility: data.compatibility,
    custom_css: data.customCss,
    asset_bundle_id: data.assetBundleId,
    image_path: data.imagePath ?? null,
    owner_id: data.ownerId ?? null,
  });

  return repo().save(theme);
}

export async function updateTheme(id: string, data: Partial<ThemeCreateData>) {
  const existing = await repo().findOneBy({ id });
  if (!existing) return null;

  if (data.imagePath && existing.image_path) {
    await deleteImageFile(existing.image_path);
  }

  repo().merge(existing, {
    ...(data.name !== undefined ? { name: data.name || String(data.config?.name || existing.name) } : {}),
    ...(data.description !== undefined ? { description: data.description } : {}),
    ...(data.config ? { config: data.config } : {}),
    ...(data.tags ? { tags: data.tags } : {}),
    ...(data.schemaVersion ? { schema_version: data.schemaVersion } : {}),
    ...(data.compatibility ? { compatibility: data.compatibility } : {}),
    ...(data.customCss !== undefined ? { custom_css: data.customCss } : {}),
    ...(data.assetBundleId !== undefined ? { asset_bundle_id: data.assetBundleId } : {}),
    ...(data.imagePath ? { image_path: data.imagePath } : {}),
  });

  return repo().save(existing);
}

export async function deleteTheme(id: string) {
  const existing = await repo().findOneBy({ id });
  if (!existing) return false;

  if (existing.image_path) {
    await deleteImageFile(existing.image_path);
  }

  await repo().remove(existing);
  return true;
}

export async function incrementDownloads(id: string) {
  const result = await repo()
    .createQueryBuilder()
    .update(Theme)
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
    .update(Theme)
    .set({ views: () => 'views + 1' })
    .where('id = :id', { id })
    .returning('views')
    .execute();

  if (result.affected === 0) return null;
  return { views: result.raw[0]?.views as number };
}

export function buildExport(theme: Theme) {
  return buildThemeExport({
    config: theme.config,
    schemaVersion: theme.schema_version,
    compatibility: theme.compatibility,
    customCss: theme.custom_css,
    assetBundleId: theme.asset_bundle_id,
  });
}

async function deleteImageFile(relativePath: string) {
  const absolute = resolveUploadPath(relativePath);
  if (!absolute) {
    logger.warn(`Refusing to delete path outside uploads root: ${relativePath}`);
    return;
  }

  try {
    await unlink(absolute);
  } catch (err) {
    logger.warn(`Could not delete theme image: ${relativePath}`, err);
  }
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
