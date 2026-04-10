import { create } from 'zustand';
import type { WorldBookSource } from '../types/worldbook';
import type { UserSettings } from '../hooks/useAuth';

interface WorldbookFilterState {
  source: WorldBookSource;
  search: string;
  sort: string;
  page: number;
  infiniteScroll: boolean;

  // Tag filters (shared across sources)
  tags: string[];
  excludeTags: string[];

  showNsfw: boolean;
  showNsfl: boolean;
  authorSearch: string;

  _hydrated: boolean;
  hydrateFromSettings: (settings: UserSettings) => void;
  setSource: (source: WorldBookSource) => void;
  setSearch: (search: string) => void;
  setSort: (sort: string) => void;
  setPage: (page: number) => void;
  addTag: (tag: string) => void;
  removeTag: (tag: string) => void;
  addExcludeTag: (tag: string) => void;
  removeExcludeTag: (tag: string) => void;
  clearTags: () => void;
  setShowNsfw: (showNsfw: boolean) => void;
  setShowNsfl: (showNsfl: boolean) => void;
  setAuthorSearch: (authorSearch: string) => void;
  setInfiniteScroll: (infiniteScroll: boolean) => void;
}

export const useWorldbookStore = create<WorldbookFilterState>((set) => ({
  source: 'chub',
  search: '',
  sort: 'trending',
  page: 1,
  infiniteScroll: false,

  tags: [],
  excludeTags: [],

  showNsfw: false,
  showNsfl: false,
  authorSearch: '',

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
    const defaultSort = source === 'lumihub' ? 'created_at' : 'trending';
    set({ source, sort: defaultSort, page: 1, tags: [], excludeTags: [], authorSearch: '' });
  },

  setSearch: (search) => set({ search, page: 1 }),

  setSort: (sort) => set({ sort, page: 1 }),

  setPage: (page) => set({ page }),

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

  setShowNsfw: (showNsfw) => set({ showNsfw, page: 1 }),

  setShowNsfl: (showNsfl) => set({ showNsfl, page: 1 }),

  setAuthorSearch: (authorSearch) => set({ authorSearch, page: 1 }),
  
  setInfiniteScroll: (infiniteScroll) => set({ infiniteScroll }),
}));
