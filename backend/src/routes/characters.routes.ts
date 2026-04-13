import { Hono } from 'hono';
import { uploadMiddleware, type UploadEnv } from '../middleware/upload.middleware.ts';
import { charxUploadMiddleware, type CharxEnv } from '../middleware/charx.middleware.ts';
import { requireAuth, type AuthEnv } from '../middleware/requireAuth.middleware.ts';
import * as CharacterService from '../services/characters.service.ts';
import type { ListQueryParams } from '../types/api.ts';

type CharacterEnv = {
  Variables: UploadEnv['Variables'] & AuthEnv['Variables'] & CharxEnv['Variables'];
};

const characters = new Hono<CharacterEnv>();

/** List public characters with pagination and filters */
characters.get('/', async (c) => {
  const query: ListQueryParams = {
    page: Number(c.req.query('page')) || undefined,
    limit: Number(c.req.query('limit')) || undefined,
    sort: c.req.query('sort') || undefined,
    order: (c.req.query('order') as 'asc' | 'desc') || undefined,
    search: c.req.query('search') || undefined,
    tags: c.req.query('tags') || undefined,
    nsfw: c.req.query('nsfw') === 'true' ? true : undefined,
    ownerId: c.req.query('ownerId') || undefined,
  };

  const result = await CharacterService.listCharacters(query);
  return c.json(result);
});

/** List distinct tags used across all characters */
characters.get('/tags', async (c) => {
  const search = c.req.query('search') || undefined;
  const tags = await CharacterService.listTags(search);
  return c.json({ tags });
});

/** Create character (requires login) */
characters.post('/', requireAuth, uploadMiddleware, async (c) => {
  const data = c.get('characterData');
  const imagePath = c.get('imagePath');
  const ownerId = c.get('userId');

  const character = await CharacterService.createCharacter(data, imagePath, ownerId);

  return c.json(
    { id: character.id, message: 'Character created successfully' },
    201,
  );
});

/** Import character from .charx file (requires login) */
characters.post('/charx', requireAuth, charxUploadMiddleware, async (c) => {
  const data = c.get('characterData');
  const imagePath = c.get('imagePath');
  const charxImages = c.get('charxImages');
  const lumiverseModules = c.get('lumiverseModules');
  const ownerId = c.get('userId');

  const character = await CharacterService.createCharacterFromCharx(
    data, imagePath, charxImages, lumiverseModules, ownerId,
  );

  return c.json({ id: character.id, message: 'Character imported from .charx' }, 201);
});

/** Export character as CCSv3 JSON card (public, no auth required). */
characters.get('/:id/card', async (c) => {
  const character = await CharacterService.getCharacterById(c.req.param('id'));
  if (!character) {
    return c.json({ error: 'Not Found', message: 'Character not found', statusCode: 404 }, 404);
  }
  if (character.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }

  // Build CCSv3-formatted card
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
      extensions: character.extensions,
      creation_date: character.creation_date,
      modification_date: character.modification_date,
    },
  };

  // Include avatar as base64 if available
  let avatarBase64: string | undefined;
  let avatarMime: string | undefined;
  if (character.image_path) {
    try {
      const { env } = await import('../env.ts');
      const path = await import('path');
      const imgPath = path.resolve(env.UPLOADS_DIR, character.image_path.replace(/^uploads\//, ''));
      const file = Bun.file(imgPath);
      if (await file.exists()) {
        const buf = await file.arrayBuffer();
        avatarBase64 = Buffer.from(buf).toString('base64');
        avatarMime = file.type || 'image/png';
      }
    } catch { /* avatar unavailable, skip */ }
  }

  return c.json({ card, avatarBase64, avatarMime });
});

/** Get all images for a character (public) */
characters.get('/:id/images', async (c) => {
  const character = await CharacterService.getCharacterById(c.req.param('id'));
  if (!character) {
    return c.json({ error: 'Not Found', message: 'Character not found', statusCode: 404 }, 404);
  }

  const images = await CharacterService.getCharacterImages(c.req.param('id'));
  return c.json({ data: images });
});

/** Export character as .charx archive (public) */
characters.get('/:id/charx', async (c) => {
  const character = await CharacterService.getCharacterById(c.req.param('id'));
  if (!character) {
    return c.json({ error: 'Not Found', message: 'Character not found', statusCode: 404 }, 404);
  }
  if (character.hidden) {
    return c.json({ error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 }, 403);
  }

  const archive = await CharacterService.buildCharxArchive(c.req.param('id'));
  if (!archive) {
    return c.json({ error: 'Server Error', message: 'Failed to build archive', statusCode: 500 }, 500);
  }

  // Count as a download
  await CharacterService.incrementDownloads(c.req.param('id')).catch(() => {});

  const filename = CharacterService.sanitizeFilename(character.name) + '.charx';
  return new Response(archive, {
    headers: {
      'Content-Type': 'application/zip',
      'Content-Disposition': `attachment; filename="${filename}"`,
    },
  });
});

/** Increment view counter */
characters.post('/:id/view', async (c) => {
  const result = await CharacterService.incrementViews(c.req.param('id'));
  if (!result) {
    return c.json({ error: 'Not Found', message: 'Character not found', statusCode: 404 }, 404);
  }
  return c.json(result);
});

/** Increment download counter */
characters.post('/:id/download', async (c) => {
  const result = await CharacterService.incrementDownloads(c.req.param('id'));

  if (!result) {
    return c.json(
      { error: 'Not Found', message: 'Character not found', statusCode: 404 },
      404,
    );
  }

  return c.json(result);
});

/** Update character */
characters.put('/:id', requireAuth, uploadMiddleware, async (c) => {
  const id = c.req.param('id');
  const userId = c.get('userId');

  const existing = await CharacterService.getCharacterById(id);
  if (!existing) {
    return c.json({ error: 'Not Found', message: 'Character not found', statusCode: 404 }, 404);
  }
  if (existing.owner_id !== userId) {
    return c.json({ error: 'Forbidden', message: 'You do not own this character', statusCode: 403 }, 403);
  }

  const data = c.get('characterData');
  const imagePath = c.get('imagePath');
  const updated = await CharacterService.updateCharacter(id, data, imagePath);

  return c.json({ data: updated, message: 'Character updated successfully' });
});

/** Delete character */
characters.delete('/:id', requireAuth, async (c) => {
  const id = c.req.param('id');
  const userId = c.get('userId');

  const existing = await CharacterService.getCharacterById(id);
  if (!existing) {
    return c.json({ error: 'Not Found', message: 'Character not found', statusCode: 404 }, 404);
  }
  if (existing.owner_id !== userId) {
    return c.json({ error: 'Forbidden', message: 'You do not own this character', statusCode: 403 }, 403);
  }

  await CharacterService.deleteCharacter(id);
  return c.json({ message: 'Character deleted successfully' });
});

/** Get full character details — MUST be last /:id route to avoid catching sub-paths */
characters.get('/:id', async (c) => {
  const character = await CharacterService.getCharacterById(c.req.param('id'));

  if (!character) {
    return c.json(
      { error: 'Not Found', message: 'Character not found', statusCode: 404 },
      404,
    );
  }

  if (character.hidden) {
    return c.json(
      { error: 'Unavailable', message: 'This content has been hidden by moderation', statusCode: 403 },
      403,
    );
  }

  return c.json({ data: character });
});

export default characters;
