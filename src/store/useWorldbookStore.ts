import { create } from 'zustand';
import type { WorldBookSource } from '../types/worldbook';
import type { UserSettings } from '../hooks/useAuth';

interface WorldbookFilterState {
  source: WorldBookSource;
  search: string;
  sort: string;

  // Tag filters (shared across sources)
  tags: string[];
  excludeTags: string[];

  showNsfw: boolean;
  showNsfl: boolean;

  _hydrated: boolean;
  hydrateFromSettings: (settings: UserSettings) => void;
  setSource: (source: WorldBookSource) => void;
  setSearch: (search: string) => void;
  setSort: (sort: string) => void;
  addTag: (tag: string) => void;
  removeTag: (tag: string) => void;
  addExcludeTag: (tag: string) => void;
  removeExcludeTag: (tag: string) => void;
  clearTags: () => void;
  setShowNsfw: (showNsfw: boolean) => void;
  setShowNsfl: (showNsfl: boolean) => void;
}

export const useWorldbookStore = create<WorldbookFilterState>((set) => ({
  source: 'chub',
  search: '',
  sort: 'default',

  tags: [],
  excludeTags: [],

  showNsfw: false,
  showNsfl: false,

  _hydrated: false,
  hydrateFromSettings: (settings) => set((s) => {
    if (s._hydrated) return s;
    return {
      _hydrated: true,
      tags: settings.defaultIncludeTags.length > 0 ? settings.defaultIncludeTags : s.tags,
      excludeTags: settings.defaultExcludeTags.length > 0 ? settings.defaultExcludeTags : s.excludeTags,
      showNsfw: settings.nsfwEnabled,
      showNsfl: settings.nsfwEnabled,
    };
  }),

  setSource: (source) => {
    const defaultSort = source === 'lumihub' ? 'created_at' : 'default';
    set({ source, sort: defaultSort, tags: [], excludeTags: [] });
  },

  setSearch: (search) => set({ search }),

  setSort: (sort) => set({ sort }),

  addTag: (tag) => set((s) => {
    const normalized = tag.trim().toLowerCase();
    if (!normalized || s.tags.includes(normalized)) return s;
    return { tags: [...s.tags, normalized] };
  }),

  removeTag: (tag) => set((s) => ({
    tags: s.tags.filter((t) => t !== tag),
  })),

  addExcludeTag: (tag) => set((s) => {
    const normalized = tag.trim().toLowerCase();
    if (!normalized || s.excludeTags.includes(normalized)) return s;
    return { excludeTags: [...s.excludeTags, normalized] };
  }),

  removeExcludeTag: (tag) => set((s) => ({
    excludeTags: s.excludeTags.filter((t) => t !== tag),
  })),

  clearTags: () => set({ tags: [], excludeTags: [] }),

  setShowNsfw: (showNsfw) => set({ showNsfw }),

  setShowNsfl: (showNsfl) => set({ showNsfl }),
}));
