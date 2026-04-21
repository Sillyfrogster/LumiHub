import { create } from 'zustand';

interface PresetFilterState {
  search: string;
  sort: string;
  page: number;
  infiniteScroll: boolean;
  tags: string[];

  setSearch: (search: string) => void;
  setSort: (sort: string) => void;
  setPage: (page: number) => void;
  addTag: (tag: string) => void;
  removeTag: (tag: string) => void;
  clearTags: () => void;
  setInfiniteScroll: (infiniteScroll: boolean) => void;
}

export const usePresetStore = create<PresetFilterState>((set) => ({
  search: '',
  sort: 'created_at',
  page: 1,
  infiniteScroll: false,
  tags: [],

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

  clearTags: () => set({ tags: [], page: 1 }),

  setInfiniteScroll: (infiniteScroll) => set({ infiniteScroll }),
}));
