import { create } from "zustand";

/** Interface state only. Server data belongs to Query. */
type UiState = {
  filtersOpen: boolean;
  toggleFilters: () => void;
};

export const useUiStore = create<UiState>((set) => ({
  filtersOpen: false,
  toggleFilters: () => set((s) => ({ filtersOpen: !s.filtersOpen })),
}));
