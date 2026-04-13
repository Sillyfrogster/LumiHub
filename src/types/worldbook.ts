import { toThumbnailUrl, toUploadUrl } from '../utils/media';

interface LumiHubOwner {
  id: string;
  discord_id: string;
  username: string;
  avatar: string | null;
}

export interface WorldBookEntry {
  keys: string[];
  content: string;
  name?: string;
  enabled: boolean;
  order?: number;
  insertion_order?: number;
  priority?: number;
  comment?: string;
  secondary_keys?: string[];
  selective?: boolean;
  constant?: boolean;
  position?: string | number;
  depth?: number;
  extensions?: Record<string, any>;
  [key: string]: unknown;
}

export function normalizeWorldbookEntries(raw: unknown): WorldBookEntry[] {
  if (Array.isArray(raw)) return raw;
  if (raw && typeof raw === 'object') {
    return Object.values(raw as Record<string, WorldBookEntry>);
  }
  return [];
}

export interface LumiWorldBook {
  id: string;
  name: string;
  description: string;
  entries: WorldBookEntry[];
  tags: string[];
  creator?: string;
  image_path: string | null;
  downloads: number;
  views: number;
  favorites: number;
  created_at: string;
  updated_at: string;
  owner?: LumiHubOwner | null;
}

export interface ChubWorldBook {
  id: string;
  fullPath: string;
  name: string;
  tagline?: string;
  description?: string;
  topics: string[];
  nsfw: boolean;
  nsfw_image: boolean;
  avatarUrl: string;
  nTokens: number;
  starCount: number;
  rating: number;
  ratingCount: number;
  favorites: number;
  forks: number;
  createdAt: string;
  lastActivityAt: string;
}

export type WorldBookSource = 'lumihub' | 'chub';

export interface UnifiedWorldBook {
  id: string;
  name: string;
  description: string;
  entryCount: number;
  tokenCount: number;
  tags: string[];
  nsfw: boolean;
  creator: string;
  creatorUsername?: string;
  creatorDiscordId?: string;
  avatarUrl: string | null;
  previewUrl?: string | null;
  downloads: number;
  views: number;
  favorites: number;
  rating: number | null;
  createdAt: string | null;
  source: WorldBookSource;
  raw: LumiWorldBook | ChubWorldBook;
}

export function fromLumiHub(wb: LumiWorldBook): UnifiedWorldBook {
  const entries = normalizeWorldbookEntries(wb.entries);

  return {
    id: wb.id,
    name: wb.name,
    description: wb.description?.slice(0, 200) || '',
    entryCount: entries.length,
    tokenCount: 0,
    tags: wb.tags,
    nsfw: wb.tags.some((t) => t.toLowerCase() === 'nsfw'),
    creator: wb.owner?.username || wb.creator || 'Unknown',
    creatorUsername: wb.owner?.username,
    creatorDiscordId: wb.owner?.discord_id,
    avatarUrl: toUploadUrl(wb.image_path),
    previewUrl: toThumbnailUrl(wb.image_path),
    downloads: wb.downloads,
    views: wb.views,
    favorites: wb.favorites,
    rating: null,
    createdAt: wb.created_at,
    source: 'lumihub',
    raw: wb,
  };
}

export function fromChubLorebook(lb: ChubWorldBook): UnifiedWorldBook {
  const creator = lb.fullPath.includes('/') ? lb.fullPath.split('/')[1] : 'Unknown';
  const hasNsfwTag = lb.topics.some((t) => t.toLowerCase() === 'nsfw');
  return {
    id: lb.fullPath,
    name: lb.name,
    description: lb.tagline || lb.description || '',
    entryCount: 0,
    tokenCount: lb.nTokens,
    tags: lb.topics,
    nsfw: lb.nsfw_image || lb.nsfw || hasNsfwTag,
    creator,
    avatarUrl: lb.avatarUrl,
    previewUrl: lb.avatarUrl,
    downloads: lb.starCount,
    views: 0,
    favorites: 0,
    rating: lb.rating ?? null,
    createdAt: lb.createdAt,
    source: 'chub',
    raw: lb,
  };
}
