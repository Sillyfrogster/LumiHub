import { useCallback } from 'react';
import CodeMirror, { type ReactCodeMirrorRef } from '@uiw/react-codemirror';
import { css } from '@codemirror/lang-css';
import { EditorView } from '@codemirror/view';
import { useProfileStudioStore } from '../../store/useProfileStudioStore';
import { useStudioContext } from './ProfileStudioContext';
import styles from './EditorTab.module.css';

const extensions = [css(), EditorView.lineWrapping];

const CssEditorTab = () => {
  const { draftCss, setDraftCss } = useProfileStudioStore();
  const { cssEditorRef } = useStudioContext();

  const onChange = useCallback(
    (value: string) => {
      setDraftCss(value);
    },
    [setDraftCss]
  );

  const onCreateEditor = useCallback(
    (view: EditorView) => {
      cssEditorRef.current = view;
    },
    [cssEditorRef]
  );

  return (
    <div className={styles.editorWrapper}>
      <CodeMirror
        value={draftCss}
        onChange={onChange}
        onCreateEditor={onCreateEditor}
        extensions={extensions}
        theme="dark"
        height="100%"
        className={styles.editor}
        placeholder="/* Style your profile here */"
      />
    </div>
  );
};

export default CssEditorTab;
