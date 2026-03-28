import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface PanelPosition {
  x: number;
  y: number;
}

interface PanelSize {
  width: number;
  height: number;
}

export type StudioTab = 'html' | 'css' | 'assets';
export type EditorMode = 'beginner' | 'advanced';

interface ProfileStudioState {
  isOpen: boolean;
  activeTab: StudioTab;
  editorMode: EditorMode;
  panelPosition: PanelPosition;
  panelSize: PanelSize;
  draftHtml: string;
  draftCss: string;
  isDirty: boolean;
  isSaving: boolean;
  isPickerActive: boolean;
  open: (html: string | null, css: string | null) => void;
  close: () => void;
  setTab: (tab: StudioTab) => void;
  setMode: (mode: EditorMode) => void;
  setPanelPosition: (pos: PanelPosition) => void;
  setPanelSize: (size: PanelSize) => void;
  setDraftHtml: (html: string) => void;
  setDraftCss: (css: string) => void;
  setSaving: (saving: boolean) => void;
  markClean: () => void;
  togglePicker: () => void;
}

const DEFAULT_POSITION: PanelPosition = { x: -1, y: 80 };
const DEFAULT_SIZE: PanelSize = { width: 480, height: 600 };

export const useProfileStudioStore = create<ProfileStudioState>()(
  persist(
    (set) => ({
      isOpen: false,
      activeTab: 'html',
      editorMode: 'beginner',
      panelPosition: DEFAULT_POSITION,
      panelSize: DEFAULT_SIZE,
      draftHtml: '',
      draftCss: '',
      isDirty: false,
      isSaving: false,
      isPickerActive: false,

      open: (html, css) => set({
        isOpen: true,
        draftHtml: html || '',
        draftCss: css || '',
        isDirty: false,
        isSaving: false,
        isPickerActive: false,
      }),

      close: () => set({
        isOpen: false,
        isPickerActive: false,
        isSaving: false,
      }),

      setTab: (tab) => set({ activeTab: tab }),
      setMode: (mode) => set({ editorMode: mode, isPickerActive: false }),

      setPanelPosition: (pos) => set({ panelPosition: pos }),
      setPanelSize: (size) => set({ panelSize: size }),

      setDraftHtml: (html) => set({ draftHtml: html, isDirty: true }),
      setDraftCss: (css) => set({ draftCss: css, isDirty: true }),

      setSaving: (saving) => set({ isSaving: saving }),
      markClean: () => set({ isDirty: false, isSaving: false }),

      togglePicker: () => set((s) => ({ isPickerActive: !s.isPickerActive })),
    }),
    {
      name: 'lumihub-profile-studio',
      partialize: (state) => ({
        activeTab: state.activeTab,
        editorMode: state.editorMode,
        panelPosition: state.panelPosition,
        panelSize: state.panelSize,
      }),
    }
  )
);
