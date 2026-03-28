export interface ChubCharacter {
  id: number;
  fullPath: string;
  name: string;
  tagline?: string;
  description?: string;
  topics?: string[];
  nsfw?: boolean;
  nsfw_image?: boolean;
  avatar_url?: string;
  max_res_url?: string;
  hasGallery?: boolean;
  nTokens?: number;
  rating?: number;
  ratingCount?: number;
  starCount?: number;
  n_favorites?: number;
  downloadCount?: number;
  nChats?: number;
  nMessages?: number;
  forksCount?: number;
  createdAt?: string;
  lastActivityAt?: string;
}

export interface ChubSearchResult {
  nodes: ChubCharacter[];
  page: number;
  hasMore: boolean;
}

export type ChubSortOption = 'default' | 'rating' | 'trending' | 'created_at' | 'download_count' | 'n_tokens';

export interface ChubSearchOptions {
  search?: string;
  page?: number;
  limit?: number;
  sort?: ChubSortOption;
  nsfw?: boolean;
  nsfl?: boolean;
  tags?: string;
  excludeTags?: string;
  minTokens?: number;
  requireImages?: boolean;
  includeForks?: boolean;
  venus?: boolean;
}

export interface ChubCharacterDefinition {
  name: string;
  description: string;
  personality: string;
  scenario: string;
  first_message: string | null;
  system_prompt: string | null;
  post_history_instructions: string | null;
  example_dialogs: string | null;
  alternate_greetings: string[];
  embedded_lorebook: {
    entries: ChubLorebookEntry[];
  } | null;
  tavern_personality: string | null;
  extensions: Record<string, unknown>;
}

export interface ChubLorebookEntry {
  keys: string[];
  secondary_keys: string[];
  content: string;
  name: string;
  comment: string;
  enabled: boolean;
  priority: number;
  insertion_order: number;
  case_sensitive: boolean;
  selective: boolean;
  constant: boolean;
  position: string;
}

export interface ChubCharacterDetail {
  node: {
    fullPath: string;
    name: string;
    description: string;
    tagline: string;
    topics: string[];
    avatar_url?: string;
    max_res_url?: string;
    starCount: number;
    nChats: number;
    nMessages: number;
    nTokens: number;
    rating: number;
    createdAt: string;
    lastActivityAt: string;
    creatorId: number;
    definition: ChubCharacterDefinition | null;
    related_lorebooks: unknown[];
  };
}

export interface ChubCharacterCard {
  id: string;
  /** Numeric Chub project ID, used for gallery API calls. */
  projectId: number;
  name: string;
  creator: string;
  tagline?: string;
  description?: string;
  tags: string[];
  nsfw: boolean;
  isNsfwImage?: boolean;
  hasGallery: boolean;
  avatarUrl: string;
  highResUrl?: string;
  pageUrl: string;
  downloadUrl: string;
  tokenCount?: number;
  rating?: number;
  ratingCount?: number;
  starCount?: number;
  favorites?: number;
  downloadCount?: number;
  chats?: number;
  messages?: number;
  forks?: number;
  createdAt?: string;
  lastActivityAt?: string;
}
