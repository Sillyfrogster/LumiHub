import { createContext, useContext, useRef, useCallback, type ReactNode } from 'react';
import type { EditorView } from '@codemirror/view';

interface ProfileStudioContextValue {
  cssEditorRef: React.MutableRefObject<EditorView | null>;
  htmlEditorRef: React.MutableRefObject<EditorView | null>;
  insertCssAtCursor: (text: string) => void;
  insertHtmlAtCursor: (text: string) => void;
}

const ProfileStudioCtx = createContext<ProfileStudioContextValue | null>(null);

export function ProfileStudioProvider({ children }: { children: ReactNode }) {
  const cssEditorRef = useRef<EditorView | null>(null);
  const htmlEditorRef = useRef<EditorView | null>(null);

  const insertCssAtCursor = useCallback((text: string) => {
    const view = cssEditorRef.current;
    if (!view) return;
    const cursor = view.state.selection.main.head;
    view.dispatch({
      changes: { from: cursor, insert: text },
      selection: { anchor: cursor + text.length },
    });
    view.focus();
  }, []);

  const insertHtmlAtCursor = useCallback((text: string) => {
    const view = htmlEditorRef.current;
    if (!view) return;
    const cursor = view.state.selection.main.head;
    view.dispatch({
      changes: { from: cursor, insert: text },
      selection: { anchor: cursor + text.length },
    });
    view.focus();
  }, []);

  return (
    <ProfileStudioCtx.Provider value={{ cssEditorRef, htmlEditorRef, insertCssAtCursor, insertHtmlAtCursor }}>
      {children}
    </ProfileStudioCtx.Provider>
  );
}

export function useStudioContext() {
  const ctx = useContext(ProfileStudioCtx);
  if (!ctx) throw new Error('useStudioContext must be used within ProfileStudioProvider');
  return ctx;
}
