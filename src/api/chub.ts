import type { ChubCharacter, ChubSearchOptions, ChubCharacterCard, ChubCharacterDetail, ChubCharacterDefinition } from '../types/chub';
import type { ChubWorldBook } from '../types/worldbook';

const CHUB_GATEWAY_BASE = (typeof import.meta.env !== 'undefined' && import.meta.env.DEV)
  ? '/api/chub'
  : 'https://gateway.chub.ai';

const CHUB_DEFAULT_PAGE_SIZE = 48;

export interface ChubSearchResult {
  nodes: ChubCharacter[];
  page: number;
  hasMore: boolean;
  total: number;
}

/** Searches for characters on Chub.ai with the given filter options. */
export async function searchChubCharacters(options: ChubSearchOptions = {}): Promise<ChubSearchResult> {
  const limit = options.limit || CHUB_DEFAULT_PAGE_SIZE;

  const params = new URLSearchParams({
    search: options.search || '',
    page: String(options.page || 1),
    first: String(limit),
    sort: options.sort || 'default',
    nsfw: String(options.nsfw ?? false),
    nsfl: String(options.nsfl ?? false),
    asc: 'false',
    min_tokens: String(options.minTokens ?? 750),
    require_images: String(options.requireImages ?? false),
    include_forks: String(options.includeForks ?? false),
    venus: String(options.venus ?? false),
  });

  if (options.tags) {
    params.append('topics', options.tags);
  }

  if (options.excludeTags) {
    params.append('excludetopics', options.excludeTags);
  }

  if (options.creator) {
    params.append('username', options.creator);
  }

  const url = `${CHUB_GATEWAY_BASE}/search?${params}`;

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Accept': 'application/json',
        'Content-Type': 'application/json',
      },
      body: '{}',
      mode: 'cors',
    });

    if (!response.ok) {
      throw new Error(`Chub API error: ${response.status}`);
    }

    const data = await response.json();
    const actualData = data.data || data;

    const nodes = actualData.nodes || [];

    return {
      nodes,
      page: options.page || 1,
      hasMore: nodes.length >= limit && (options.page || 1) < 100, // Capping pages at 100 max
      total: actualData.count || actualData.total || 0,
    };
  } catch (error) {
    console.error('[LumiHub] Chub API error:', error);
    throw error;
  }
}

/** Transforms a raw Chub API node into the app's ChubCharacterCard format. */
export function transformChubCharacter(node: ChubCharacter): ChubCharacterCard {
  const fullPath = node.fullPath || node.name;
  const creator = fullPath.includes('/') ? fullPath.split('/')[0] : 'Unknown';

  const hasNsfwTag = (node.topics || []).some(t => t.toLowerCase() === 'nsfw');
  const isNsfw = node.nsfw_image || node.nsfw || hasNsfwTag;

  return {
    id: fullPath,
    projectId: node.id,
    name: node.name || 'Unnamed',
    creator,
    tagline: node.tagline,
    description: node.description,
    tags: node.topics || [],
    nsfw: isNsfw,
    isNsfwImage: node.nsfw_image || false,
    hasGallery: node.hasGallery || false,
    avatarUrl: node.max_res_url || node.avatar_url || `https://avatars.charhub.io/avatars/${fullPath}/avatar.webp`,
    highResUrl: `https://avatars.charhub.io/avatars/${fullPath}/chara_card_v2.png`,
    pageUrl: `https://chub.ai/characters/${fullPath}`,
    downloadUrl: `https://avatars.charhub.io/avatars/${fullPath}/chara_card_v2.png`,
    tokenCount: node.nTokens,
    rating: node.rating,
    ratingCount: node.ratingCount,
    starCount: node.starCount,
    favorites: node.n_favorites,
    downloadCount: node.downloadCount,
    chats: node.nChats,
    messages: node.nMessages,
    forks: node.forksCount,
    createdAt: node.createdAt,
    lastActivityAt: node.lastActivityAt,
  };
}

/** Fetches the full character definition from Chub.ai (prompts, greetings, lorebook). */
export async function getChubCharacterDetail(fullPath: string): Promise<ChubCharacterDefinition | null> {
  const url = `${CHUB_GATEWAY_BASE}/api/characters/${fullPath}?full=true`;

  try {
    const response = await fetch(url, {
      headers: { 'Accept': 'application/json' },
      mode: 'cors',
    });

    if (!response.ok) {
      throw new Error(`Chub API error: ${response.status}`);
    }

    const data: ChubCharacterDetail = await response.json();
    return data.node?.definition ?? null;
  } catch (error) {
    console.error('[LumiHub] Chub character detail error:', error);
    throw error;
  }
}

/** Fetches trending characters from Chub.ai. */
export async function getTrendingCharacters(limit = 24, includeNsfw = false): Promise<ChubCharacterCard[]> {
  const result = await searchChubCharacters({
    sort: 'trending',
    limit,
    page: 1,
    nsfw: includeNsfw,
  });

  return result.nodes.map(transformChubCharacter);
}

/** Fetches top-rated characters from Chub.ai. */
export async function getFeaturedCharacters(limit = 24, includeNsfw = false): Promise<ChubCharacterCard[]> {
  const result = await searchChubCharacters({
    sort: 'rating',
    limit,
    page: 1,
    nsfw: includeNsfw,
  });

  return result.nodes.map(transformChubCharacter);
}

// ── Lorebook / Worldbook API ──────────────────────────────────

export interface ChubLorebookSearchOptions {
  search?: string;
  page?: number;
  limit?: number;
  sort?: string;
  nsfw?: boolean;
  nsfl?: boolean;
  tags?: string;
  excludeTags?: string;
  creator?: string;
}

export interface ChubLorebookSearchResult {
  nodes: ChubWorldBook[];
  page: number;
  hasMore: boolean;
  total: number;
}

/** Searches for lorebooks on Chub.ai. */
export async function searchChubLorebooks(options: ChubLorebookSearchOptions = {}): Promise<ChubLorebookSearchResult> {
  const limit = options.limit || CHUB_DEFAULT_PAGE_SIZE;

  const params = new URLSearchParams({
    search: options.search || '',
    page: String(options.page || 1),
    first: String(limit),
    sort: options.sort || 'default',
    nsfw: String(options.nsfw ?? false),
    nsfl: String(options.nsfl ?? false),
    asc: 'false',
    namespace: 'lorebooks',
  });

  if (options.tags) {
    params.append('topics', options.tags);
  }

  if (options.excludeTags) {
    params.append('excludetopics', options.excludeTags);
  }

  if (options.creator) {
    params.append('username', options.creator);
  }

  const url = `${CHUB_GATEWAY_BASE}/search?${params}`;

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Accept': 'application/json',
        'Content-Type': 'application/json',
      },
      body: '{}',
      mode: 'cors',
    });

    if (!response.ok) {
      throw new Error(`Chub API error: ${response.status}`);
    }

    const data = await response.json();
    const actualData = data.data || data;
    const rawNodes = actualData.nodes || [];

    const nodes: ChubWorldBook[] = rawNodes.map((node: ChubCharacter) => {
      const fullPath = node.fullPath || node.name;
      return {
        id: fullPath,
        fullPath,
        name: node.name || 'Unnamed',
        tagline: node.tagline,
        description: node.description,
        topics: node.topics || [],
        nsfw: !!(node as any).nsfw,
        nsfw_image: !!(node as any).nsfw_image,
        avatarUrl: node.max_res_url || node.avatar_url || `https://avatars.charhub.io/avatars/${fullPath}/avatar.webp`,
        nTokens: node.nTokens ?? 0,
        starCount: node.starCount ?? 0,
        rating: node.rating ?? 0,
        ratingCount: node.ratingCount ?? 0,
        favorites: node.n_favorites ?? 0,
        forks: node.forksCount ?? 0,
        createdAt: node.createdAt ?? '',
        lastActivityAt: node.lastActivityAt ?? '',
      };
    });

    return {
      nodes,
      page: options.page || 1,
      hasMore: rawNodes.length >= limit,
      total: actualData.count || actualData.total || 0,
    };
  } catch (error) {
    console.error('[LumiHub] Chub lorebook search error:', error);
    throw error;
  }
}

// ── Creator profile ─────────────────────────────────────────

export interface ChubCreatorProfile {
  username: string;
  displayName: string;
  avatarUrl: string | null;
  bio: string;
  followers: number;
  following: number;
  projectCount: number;
  /** Top characters by star count, already transformed for LumiHub navigation. */
  topCharacters: ChubCharacterCard[];
}

/** Fetches a Chub creator's public profile by username. */
export async function getChubCreatorProfile(username: string): Promise<ChubCreatorProfile | null> {
  const url = `${CHUB_GATEWAY_BASE}/api/users/${encodeURIComponent(username)}`;

  try {
    const response = await fetch(url, {
      headers: { 'Accept': 'application/json' },
      mode: 'cors',
    });

    if (!response.ok) return null;

    const data = await response.json();

    const projects = data.projects?.nodes ?? [];
    // Sort by stars descending and take top 5
    const topNodes = [...projects]
      .sort((a: any, b: any) => (b.starCount ?? 0) - (a.starCount ?? 0))
      .slice(0, 5);

    return {
      username: data.username ?? username,
      displayName: data.name || data.username || username,
      avatarUrl: data.avatar_url || null,
      bio: data.bio ?? '',
      followers: data.n_followers ?? 0,
      following: data.n_followees ?? 0,
      projectCount: data.projects?.count ?? projects.length,
      topCharacters: topNodes.map(transformChubCharacter),
    };
  } catch {
    return null;
  }
}

// ── Tag listing ──────────────────────────────────────────────

export interface ChubTag {
  name: string;
  count: number;
}

/** Fetches available tags from Chub.ai with optional search filtering. */
export async function fetchChubTags(search?: string, namespace?: string): Promise<ChubTag[]> {
  const url = `${CHUB_GATEWAY_BASE}/tags`;

  try {
    const body: Record<string, any> = {
      search: search || '',
      first: 50
    };
    if (namespace) body.namespace = namespace;

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Accept': 'application/json',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
      mode: 'cors',
    });

    if (!response.ok) {
      throw new Error(`Chub tags API error: ${response.status}`);
    }

    const data = await response.json();
    const tags = (data.tags || []) as Array<{
      name: string;
      non_private_projects_count: number;
    }>;

    return tags
      .filter((t) => t.name && t.name !== 'ROOT')
      .map((t) => ({
        name: t.name,
        count: t.non_private_projects_count ?? 0,
      }));
  } catch (error) {
    console.error('[LumiHub] Chub tags API error:', error);
    return [];
  }
}

// ── Gallery ─────────────────────────────────────────────────

export interface ChubGalleryImage {
  uuid: string;
  primary_image_path: string;
  nsfw_image: boolean;
  description: string | null;
}

/** Fetches gallery images for a Chub character by its numeric project ID. */
export async function getChubCharacterGallery(projectId: number): Promise<ChubGalleryImage[]> {
  const url = `${CHUB_GATEWAY_BASE}/api/gallery/project/${projectId}`;

  try {
    const response = await fetch(url, {
      headers: { 'Accept': 'application/json' },
      mode: 'cors',
    });

    if (!response.ok) return [];

    const data = await response.json();
    const nodes: ChubGalleryImage[] = (data.nodes || []).map((n: any) => ({
      uuid: n.uuid,
      primary_image_path: n.primary_image_path,
      nsfw_image: n.nsfw_image ?? false,
      description: n.description ?? null,
    }));
    return nodes;
  } catch {
    return [];
  }
}

/** Fetches the full lorebook definition (entries) from Chub.ai. */
export async function getChubLorebookDetail(fullPath: string): Promise<ChubCharacterDefinition | null> {
  // fullPath is "lorebooks/creator/slug" — strip the "lorebooks/" prefix for the API
  const apiPath = fullPath.replace(/^lorebooks\//, '');
  const url = `${CHUB_GATEWAY_BASE}/api/lorebooks/${apiPath}?full=true`;

  try {
    const response = await fetch(url, {
      headers: { 'Accept': 'application/json' },
      mode: 'cors',
    });

    if (!response.ok) {
      throw new Error(`Chub API error: ${response.status}`);
    }

    const data: ChubCharacterDetail = await response.json();
    return data.node?.definition ?? null;
  } catch (error) {
    console.error('[LumiHub] Chub lorebook detail error:', error);
    throw error;
  }
}
