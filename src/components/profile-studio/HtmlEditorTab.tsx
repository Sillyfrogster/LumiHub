import { useCallback } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { html } from '@codemirror/lang-html';
import { EditorView } from '@codemirror/view';
import { useProfileStudioStore } from '../../store/useProfileStudioStore';
import { useStudioContext } from './ProfileStudioContext';
import styles from './EditorTab.module.css';

const extensions = [html(), EditorView.lineWrapping];

const HtmlEditorTab = () => {
  const { draftHtml, setDraftHtml } = useProfileStudioStore();
  const { htmlEditorRef } = useStudioContext();

  const onChange = useCallback(
    (value: string) => {
      setDraftHtml(value);
    },
    [setDraftHtml]
  );

  const onCreateEditor = useCallback(
    (view: EditorView) => {
      htmlEditorRef.current = view;
    },
    [htmlEditorRef]
  );

  return (
    <div className={styles.editorWrapper}>
      <CodeMirror
        value={draftHtml}
        onChange={onChange}
        onCreateEditor={onCreateEditor}
        extensions={extensions}
        theme="dark"
        height="100%"
        className={styles.editor}
        placeholder="<!-- Write your profile HTML here -->"
      />
    </div>
  );
};

export default HtmlEditorTab;
