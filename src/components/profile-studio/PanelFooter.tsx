import { useState } from 'react';
import { Save, RotateCcw, Loader2, HelpCircle } from 'lucide-react';
import { useProfileStudioStore } from '../../store/useProfileStudioStore';
import EditorGuide from './EditorGuide';
import styles from './PanelFooter.module.css';

interface PanelFooterProps {
  onSave: () => void;
  onReset: () => void;
}

const PanelFooter = ({ onSave, onReset }: PanelFooterProps) => {
  const { isDirty, isSaving } = useProfileStudioStore();
  const [showResetConfirm, setShowResetConfirm] = useState(false);
  const [showGuide, setShowGuide] = useState(false);

  return (
    <div className={styles.footer}>
      <div className={styles.status}>
        {isSaving ? (
          <span className={styles.statusSaving}>
            <Loader2 size={12} className={styles.spinner} /> Saving...
          </span>
        ) : isDirty ? (
          <span className={styles.statusDirty}>Unsaved changes</span>
        ) : (
          <span className={styles.statusClean}>Saved</span>
        )}
      </div>

      <div className={styles.actions}>
        {showResetConfirm ? (
          <div className={styles.confirmGroup}>
            <span className={styles.confirmText}>Reset everything?</span>
            <button
              className={styles.confirmYes}
              onClick={() => {
                onReset();
                setShowResetConfirm(false);
              }}
            >
              Yes
            </button>
            <button
              className={styles.confirmNo}
              onClick={() => setShowResetConfirm(false)}
            >
              No
            </button>
          </div>
        ) : (
          <button
            className={styles.resetBtn}
            onClick={() => setShowResetConfirm(true)}
            title="Reset to Default"
          >
            <RotateCcw size={14} />
          </button>
        )}

        <button
          className={styles.guideBtn}
          onClick={() => setShowGuide(true)}
          title="Editor Guide"
        >
          <HelpCircle size={14} />
        </button>

        <button
          className={styles.saveBtn}
          onClick={onSave}
          disabled={!isDirty || isSaving}
        >
          <Save size={14} />
          Save
        </button>
      </div>

      {showGuide && <EditorGuide onClose={() => setShowGuide(false)} />}
    </div>
  );
};

export default PanelFooter;
