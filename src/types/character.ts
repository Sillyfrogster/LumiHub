import type { ChubCharacterCard } from './chub';
import { toThumbnailUrl, toUploadUrl } from '../utils/media';

export interface CharacterAsset {
  type: string;
  uri: string;
  name: string;
  ext: string;
}

/** Mirrors the backend Character entity shape. */
export interface LumiHubCharacter {
  id: string;
  name: string;
  nickname: string | null;
  description: string;
  personality: string;
  scenario: string;
  first_mes: string;
  alternate_greetings: string[];
  group_only_greetings: string[];
  mes_example: string;
  creator: string;
  creator_notes: string;
  creator_notes_multilingual: Record<string, string> | null;
  tags: string[];
  character_version: string;
  system_prompt: string;
  post_history_instructions: string;
  source: string[] | null;
  assets: CharacterAsset[];
  character_book: unknown | null;
  extensions: Record<string, unknown>;
  creation_date: number | null;
  modification_date: number | null;
  image_path: string | null;
  downloads: number;
  views: number;
  favorites: number;
  owner?: { id: string, discord_id: string, username: string, avatar: string | null } | null;
  created_at: string;
  updated_at: string;
}

export interface CharacterImage {
  id: string;
  character_id: string;
  image_type: 'avatar' | 'avatar_alt' | 'expression' | 'gallery';
  label: string | null;
  file_path: string;
  mime_type: string;
  file_size: number;
  sort_order: number;
  created_at: string;
}

export type CharacterSource = 'lumihub' | 'chub';

/** Normalized display card used across the UI regardless of source. */
export interface UnifiedCharacterCard {
  id: string;
  name: string;
  creator: string;
  creatorUsername?: string;
  creatorDiscordId?: string;
  tagline: string;
  tags: string[];
  nsfw: boolean;
  avatarUrl: string | null;
  previewUrl?: string | null;
  downloads: number;
  views: number;
  stars?: number;
  favorites?: number;
  rating: number | null;
  createdAt: string | null;
  source: CharacterSource;
  raw: LumiHubCharacter | ChubCharacterCard;
}

/** Converts a LumiHub backend character into a UnifiedCharacterCard. */
export function fromLumiHub(char: LumiHubCharacter): UnifiedCharacterCard {
  return {
    id: char.id,
    name: char.name,
    creator: char.owner?.username || char.creator || 'Unknown',
    creatorUsername: char.owner?.username,
    creatorDiscordId: char.owner?.discord_id,
    tagline: char.description?.slice(0, 200) || char.creator_notes || '',
    tags: char.tags,
    nsfw: char.tags.some((t) => t.toLowerCase() === 'nsfw'),
    avatarUrl: toUploadUrl(char.image_path),
    previewUrl: toThumbnailUrl(char.image_path),
    downloads: char.downloads,
    views: char.views,
    favorites: char.favorites,
    rating: null,
    createdAt: char.created_at,
    source: 'lumihub',
    raw: char,
  };
}

/** Converts a Chub character card into a UnifiedCharacterCard. */
export function fromChub(card: ChubCharacterCard): UnifiedCharacterCard {
  return {
    id: card.id,
    name: card.name,
    creator: card.creator,
    tagline: card.tagline || card.description || '',
    tags: card.tags,
    nsfw: card.nsfw,
    avatarUrl: card.avatarUrl,
    previewUrl: card.avatarUrl,
    downloads: card.downloadCount ?? 0,
    stars: card.starCount ?? 0,
    favorites: card.favorites ?? 0,
    rating: card.rating ?? null,
    createdAt: card.createdAt ?? null,
    source: 'chub',
    raw: card,
  };
}
