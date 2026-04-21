import { toThumbnailUrl, toUploadUrl } from '../utils/media';

interface LumiHubOwner {
  id: string;
  discord_id: string;
  username: string;
  avatar: string | null;
}

export interface LumiPreset {
  id: string;
  name: string;
  description: string;
  tags: string[];
  settings: Record<string, any>;
  image_path: string | null;
  downloads: number;
  views: number;
  favorites: number;
  created_at: string;
  updated_at: string;
  owner?: LumiHubOwner | null;
}

export interface UnifiedPreset {
  id: string;
  name: string;
  description: string;
  tags: string[];
  settings: Record<string, any>;
  creator: string;
  creatorUsername?: string;
  creatorDiscordId?: string;
  avatarUrl: string | null;
  previewUrl?: string | null;
  downloads: number;
  views: number;
  favorites: number;
  createdAt: string | null;
  raw: LumiPreset;
}

export function fromLumiHub(preset: LumiPreset): UnifiedPreset {
  return {
    id: preset.id,
    name: preset.name,
    description: preset.description && preset.description.length > 200
      ? `${preset.description.slice(0, 200)}…`
      : preset.description || '',
    tags: preset.tags,
    settings: preset.settings,
    creator: preset.owner?.username || 'Unknown',
    creatorUsername: preset.owner?.username,
    creatorDiscordId: preset.owner?.discord_id,
    avatarUrl: toUploadUrl(preset.image_path),
    previewUrl: toThumbnailUrl(preset.image_path),
    downloads: preset.downloads,
    views: preset.views,
    favorites: preset.favorites,
    createdAt: preset.created_at,
    raw: preset,
  };
}
