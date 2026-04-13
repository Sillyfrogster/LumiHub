import { unlink } from 'node:fs/promises';
import path from 'node:path';
import { zipSync } from 'fflate';
import { AppDataSource } from '../db/connection.ts';
import { Character } from '../entities/Character.entity.ts';
import { CharacterImage } from '../entities/CharacterImage.entity.ts';
import { logger } from '../utils/logger.ts';
import { UPLOAD_PATHS } from '../utils/constants.ts';
import type { ValidatedCharacterData } from '../middleware/upload.middleware.ts';
import type { CharxImageEntry, LumiverseModules } from '../middleware/charx.middleware.ts';
import type { ListQueryParams } from '../types/api.ts';

const repo = () => AppDataSource.getRepository(Character);
const imageRepo = () => AppDataSource.getRepository(CharacterImage);

/** Returns a paginated, filterable, and sortable list of characters. */
export async function listCharacters(params: ListQueryParams) {
  const page = Math.max(params.page ?? 1, 1);
  const limit = Math.min(Math.max(params.limit ?? 20, 1), 100);
  const skip = (page - 1) * limit;
  const ALLOWED_SORT_FIELDS = ['created_at', 'updated_at', 'downloads', 'views', 'rating', 'name'];
  const sortField = ALLOWED_SORT_FIELDS.includes(params.sort ?? '') ? params.sort! : 'created_at';
  const sortOrder = (params.order === 'asc' ? 'ASC' : 'DESC') as 'ASC' | 'DESC';

  const qb = repo().createQueryBuilder('character')
    .leftJoinAndSelect('character.owner', 'owner')
    .where('character.hidden = false')
    .orderBy(`character.${sortField}`, sortOrder)
    .skip(skip)
    .take(limit);

  if (params.search) {
    qb.andWhere('character.name ILIKE :search', { search: `%${params.search}%` });
  }

  if (params.ownerId) {
    qb.andWhere('character.owner_id = :ownerId', { ownerId: params.ownerId });
  }

  if (params.tags) {
    const tagArray = params.tags.split(',').map(t => t.trim().toLowerCase()).filter(Boolean);
    if (tagArray.length > 0) {
      qb.andWhere(
        `EXISTS (SELECT 1 FROM jsonb_array_elements_text(character.tags) AS t WHERE LOWER(t) IN (:...tagValues))`,
        { tagValues: tagArray },
      );
    }
  }

  const [characters, total] = await qb.getManyAndCount();

  return {
    data: characters,
    pagination: {
      page,
      limit,
      total,
      totalPages: Math.ceil(total / limit),
    },
  };
}

/** Returns distinct tags used across all characters, with usage counts. */
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
    FROM characters, jsonb_array_elements_text(tags) AS tag
    ${where} ${hiddenClause}
    GROUP BY tag
    ORDER BY count DESC
    LIMIT 200
  `;

  const rows: { name: string; count: number }[] =
    await AppDataSource.query(sql, params);
  return rows;
}

/** Finds a single character by its UUID. */
export async function getCharacterById(id: string) {
  return repo().findOne({ where: { id }, relations: ['owner'] });
}

/** Creates and persists a new character entity. */
export async function createCharacter(
  data: ValidatedCharacterData,
  imagePath: string | undefined,
  ownerId?: string | null,
) {
  const character = repo().create({
    ...data,
    image_path: imagePath ?? null,
    owner_id: ownerId ?? null,
    nickname: data.nickname ?? null,
    creator_notes_multilingual: data.creator_notes_multilingual ?? null,
    source: data.source ?? null,
    character_book: data.character_book ?? null,
    creation_date: data.creation_date ?? null,
    modification_date: data.modification_date ?? null,
  });

  return repo().save(character);
}

/** Updates an existing character, replacing the image if a new one is provided. */
export async function updateCharacter(
  id: string,
  data: Partial<ValidatedCharacterData>,
  imagePath: string | undefined,
) {
  const existing = await repo().findOneBy({ id });

  if (!existing) return null;

  if (imagePath && existing.image_path) {
    await deleteImageFile(existing.image_path);
  }

  repo().merge(existing, {
    ...data,
    ...(imagePath ? { image_path: imagePath } : {}),
    modification_date: Math.floor(Date.now() / 1000),
  });

  return repo().save(existing);
}

/** Deletes a character and removes all associated image files from disk. */
export async function deleteCharacter(id: string) {
  const existing = await repo().findOneBy({ id });

  if (!existing) return false;

  // Delete associated CharacterImage files (CASCADE handles DB rows)
  const images = await imageRepo().find({ where: { character_id: id } });
  for (const img of images) {
    await deleteImageFile(img.file_path);
  }

  if (existing.image_path) {
    await deleteImageFile(existing.image_path);
  }

  await repo().remove(existing);
  return true;
}

/** Atomically increments the download counter for a character. */
export async function incrementDownloads(id: string) {
  const result = await repo()
    .createQueryBuilder()
    .update(Character)
    .set({ downloads: () => 'downloads + 1' })
    .where('id = :id', { id })
    .returning('downloads')
    .execute();

  if (result.affected === 0) return null;

  return { downloads: result.raw[0]?.downloads as number };
}

/** Atomically increments the view counter for a character. */
export async function incrementViews(id: string) {
  const result = await repo()
    .createQueryBuilder()
    .update(Character)
    .set({ views: () => 'views + 1' })
    .where('id = :id', { id })
    .returning('views')
    .execute();

  if (result.affected === 0) return null;

  return { views: result.raw[0]?.views as number };
}

/** Creates a character from a .charx import with all associated images. */
export async function createCharacterFromCharx(
  data: ValidatedCharacterData,
  imagePath: string | undefined,
  charxImages: CharxImageEntry[],
  lumiverseModules: LumiverseModules | null,
  ownerId?: string | null,
) {
  // Merge lumiverse_modules into extensions
  const extensions = { ...data.extensions };
  if (lumiverseModules) {
    extensions.lumiverse_modules = lumiverseModules;
  }

  const character = repo().create({
    ...data,
    extensions,
    image_path: imagePath ?? null,
    owner_id: ownerId ?? null,
    nickname: data.nickname ?? null,
    creator_notes_multilingual: data.creator_notes_multilingual ?? null,
    source: data.source ?? null,
    character_book: data.character_book ?? null,
    creation_date: data.creation_date ?? null,
    modification_date: data.modification_date ?? null,
  });

  const saved = await repo().save(character);

  // Bulk-insert character images
  if (charxImages.length > 0) {
    const images = charxImages.map((entry, i) =>
      imageRepo().create({
        character_id: saved.id,
        image_type: entry.type,
        label: entry.label,
        file_path: entry.filePath,
        mime_type: entry.mimeType,
        file_size: entry.fileSize,
        sort_order: i,
      }),
    );
    await imageRepo().save(images);
  }

  return saved;
}

/** Fetches all images for a character, optionally filtered by type. */
export async function getCharacterImages(
  characterId: string,
  type?: CharacterImage['image_type'],
) {
  const where: Record<string, any> = { character_id: characterId };
  if (type) where.image_type = type;

  return imageRepo().find({
    where,
    order: { sort_order: 'ASC' },
  });
}

/** Checks if a character has any CharacterImage entries beyond just the primary avatar. */
export async function hasCharxAssets(characterId: string): Promise<boolean> {
  const count = await imageRepo().count({ where: { character_id: characterId } });
  return count > 1;
}

/** MIME extension map for archive building. */
const EXT_FROM_MIME: Record<string, string> = {
  'image/png': '.png',
  'image/jpeg': '.jpg',
  'image/gif': '.gif',
  'image/webp': '.webp',
  'image/avif': '.avif',
  'image/bmp': '.bmp',
  'image/svg+xml': '.svg',
};

/** Sanitize a string for use as a filename inside the archive. */
function sanitizeArchiveName(name: string): string {
  return name.replace(/[^a-zA-Z0-9_\-. ]/g, '_').trim() || 'unnamed';
}

/** Builds a .charx ZIP archive from a character and its stored images. */
export async function buildCharxArchive(characterId: string): Promise<Uint8Array | null> {
  const character = await repo().findOneBy({ id: characterId });
  if (!character) return null;

  const images = await getCharacterImages(characterId);
  const entries: Record<string, Uint8Array> = {};

  // Build card.json
  const card = {
    spec: 'chara_card_v3',
    spec_version: '3.0',
    data: {
      name: character.name,
      description: character.description,
      personality: character.personality,
      scenario: character.scenario,
      first_mes: character.first_mes,
      mes_example: character.mes_example,
      alternate_greetings: character.alternate_greetings,
      group_only_greetings: character.group_only_greetings,
      system_prompt: character.system_prompt,
      post_history_instructions: character.post_history_instructions,
      creator: character.creator,
      creator_notes: character.creator_notes,
      creator_notes_multilingual: character.creator_notes_multilingual,
      character_version: character.character_version,
      tags: character.tags,
      nickname: character.nickname,
      source: character.source,
      assets: character.assets,
      character_book: character.character_book,
      // Strip internal keys from extensions
      extensions: Object.fromEntries(
        Object.entries(character.extensions || {}).filter(([k]) => k !== 'lumiverse_modules'),
      ),
      creation_date: character.creation_date,
      modification_date: character.modification_date,
    },
  };
  entries['card.json'] = new TextEncoder().encode(JSON.stringify(card, null, 2));

  // Rebuild lumiverse_modules for the archive
  const storedModules = character.extensions?.lumiverse_modules as LumiverseModules | undefined;
  const modules: Record<string, any> = storedModules
    ? { version: storedModules.version ?? 1 }
    : { version: 1 };

  // Read and pack each image
  const exprMappings: Record<string, string> = {};
  const altAvatars: Array<{ id: string; label: string; path: string }> = [];

  for (const img of images) {
    const ext = EXT_FROM_MIME[img.mime_type] || path.extname(img.file_path) || '.png';

    try {
      const file = Bun.file(path.resolve(img.file_path));
      if (!(await file.exists())) continue;
      const bytes = new Uint8Array(await file.arrayBuffer());

      let archivePath: string;

      switch (img.image_type) {
        case 'avatar':
          archivePath = `assets/icon/image/main${ext}`;
          break;
        case 'avatar_alt':
          archivePath = `assets/icon/image/${img.id}${ext}`;
          altAvatars.push({ id: img.id, label: img.label || img.id, path: archivePath });
          break;
        case 'expression':
          archivePath = `assets/other/image/expr_${sanitizeArchiveName(img.label || 'unnamed')}${ext}`;
          exprMappings[img.label || 'unnamed'] = archivePath;
          break;
        case 'gallery':
          archivePath = `assets/other/image/gallery_${img.id}${ext}`;
          break;
        default:
          continue;
      }

      entries[archivePath] = bytes;
    } catch (err) {
      logger.warn(`Could not read image for charx export: ${img.file_path}`, err);
    }
  }

  // Build lumiverse_modules.json if we have extra assets
  if (Object.keys(exprMappings).length > 0 && storedModules?.expressions) {
    modules.expressions = {
      enabled: storedModules.expressions.enabled,
      defaultExpression: storedModules.expressions.defaultExpression,
      mappings: exprMappings,
    };
    if (storedModules.has_nsfw_expressions) {
      modules.has_nsfw_expressions = true;
    }
  }
  if (storedModules?.alternate_fields) {
    modules.alternate_fields = storedModules.alternate_fields;
  }
  if (altAvatars.length > 0) {
    modules.alternate_avatars = altAvatars;
  }
  if (storedModules?.regex_scripts && Array.isArray(storedModules.regex_scripts) && storedModules.regex_scripts.length > 0) {
    modules.regex_scripts = storedModules.regex_scripts;
  }

  const hasModules = modules.expressions || modules.alternate_fields || modules.alternate_avatars || modules.regex_scripts;
  if (hasModules) {
    entries['lumiverse_modules.json'] = new TextEncoder().encode(JSON.stringify(modules, null, 2));
  }

  return zipSync(entries);
}

/** Sanitize a character name for use in Content-Disposition filenames. */
export function sanitizeFilename(name: string): string {
  return name.replace(/[^a-zA-Z0-9_\-. ]/g, '_').trim() || 'character';
}

/** Removes an image file from the filesystem. */
async function deleteImageFile(relativePath: string) {
  try {
    const absolute = resolveUploadPath(relativePath);
    if (!absolute) {
      logger.warn(`Refusing to delete path outside uploads root: ${relativePath}`);
      return;
    }
    await unlink(absolute);
    logger.info(`Deleted image file: ${absolute}`);
  } catch (err) {
    logger.warn(`Could not delete image file: ${relativePath}`, err);
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
