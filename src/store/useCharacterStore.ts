import { create } from 'zustand';
import type { CharacterSource } from '../types/character';
import type { UserSettings } from '../hooks/useAuth';

interface CharacterFilterState {
  source: CharacterSource;
  search: string;
  sort: string;
  page: number;

  // Tag filters (shared across sources)
  tags: string[];
  excludeTags: string[];

  // Chub-specific filters
  minTokens: number;
  showNsfw: boolean;
  showNsfl: boolean;
  requireImages: boolean;

  _hydrated: boolean;
  hydrateFromSettings: (settings: UserSettings) => void;
  setSource: (source: CharacterSource) => void;
  setSearch: (search: string) => void;
  setSort: (sort: string) => void;
  setPage: (page: number) => void;
  addTag: (tag: string) => void;
  removeTag: (tag: string) => void;
  addExcludeTag: (tag: string) => void;
  removeExcludeTag: (tag: string) => void;
  clearTags: () => void;
  setMinTokens: (minTokens: number) => void;
  setShowNsfw: (showNsfw: boolean) => void;
  setShowNsfl: (showNsfl: boolean) => void;
  setRequireImages: (requireImages: boolean) => void;
}

/** Stores filter UI state for characters, delegating actual fetching to React Query */
export const useCharacterStore = create<CharacterFilterState>((set) => ({
  source: 'lumihub',
  search: '',
  sort: 'created_at',
  page: 1,

  tags: [],
  excludeTags: [],

  minTokens: 750,
  showNsfw: false,
  showNsfl: false,
  requireImages: false,

  _hydrated: false,
  hydrateFromSettings: (settings) => set((s) => {
    if (s._hydrated) return s;
    return {
      _hydrated: true,
      tags: settings.defaultIncludeTags.length > 0 ? settings.defaultIncludeTags : s.tags,
      excludeTags: settings.defaultExcludeTags.length > 0 ? settings.defaultExcludeTags : s.excludeTags,
      showNsfw: settings.nsfwEnabled,
      showNsfl: settings.nsfwEnabled,
      page: 1,
    };
  }),

  setSource: (source) => {
    const defaultSort = source === 'lumihub' ? 'created_at' : 'default';
    set({ source, sort: defaultSort, page: 1, tags: [], excludeTags: [] });
  },

  setSearch: (search) => {
    set({ search, page: 1 });
  },

  setSort: (sort) => {
    set({ sort, page: 1 });
  },

  setPage: (page) => {
    set({ page });
  },

  addTag: (tag) => set((s) => {
    const normalized = tag.trim().toLowerCase();
    if (!normalized || s.tags.includes(normalized)) return s;
    return { tags: [...s.tags, normalized], page: 1 };
  }),

  removeTag: (tag) => set((s) => ({
    tags: s.tags.filter((t) => t !== tag),
    page: 1,
  })),

  addExcludeTag: (tag) => set((s) => {
    const normalized = tag.trim().toLowerCase();
    if (!normalized || s.excludeTags.includes(normalized)) return s;
    return { excludeTags: [...s.excludeTags, normalized], page: 1 };
  }),

  removeExcludeTag: (tag) => set((s) => ({
    excludeTags: s.excludeTags.filter((t) => t !== tag),
    page: 1,
  })),

  clearTags: () => set({ tags: [], excludeTags: [], page: 1 }),

  setMinTokens: (minTokens) => {
    set({ minTokens, page: 1 });
  },

  setShowNsfw: (showNsfw) => {
    set({ showNsfw, page: 1 });
  },

  setShowNsfl: (showNsfl) => {
    set({ showNsfl, page: 1 });
  },

  setRequireImages: (requireImages) => {
    set({ requireImages, page: 1 });
  },
}));
