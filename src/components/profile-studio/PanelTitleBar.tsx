import { forwardRef } from 'react';
import { X, Minus, Wand2, Code, Crosshair } from 'lucide-react';
import { useProfileStudioStore } from '../../store/useProfileStudioStore';
import ElementListDropdown from './ElementListDropdown';
import styles from './PanelTitleBar.module.css';

interface PanelTitleBarProps {
  onClose: () => void;
  onMinimize: () => void;
  onSelectElement: (element: HTMLElement) => void;
}

const PanelTitleBar = forwardRef<HTMLDivElement, PanelTitleBarProps>(
  ({ onClose, onMinimize, onSelectElement }, ref) => {
    const { editorMode, setMode, isPickerActive, togglePicker } = useProfileStudioStore();

    return (
      <div className={styles.titleBar} ref={ref}>
        <span className={styles.title}>Profile Studio</span>

        <div className={styles.controls}>
          {/* Mode toggle */}
          <div className={styles.modeToggle}>
            <button
              className={`${styles.modeBtn} ${editorMode === 'beginner' ? styles.modeBtnActive : ''}`}
              onClick={() => setMode('beginner')}
              title="Visual Mode"
            >
              <Wand2 size={14} />
            </button>
            <button
              className={`${styles.modeBtn} ${editorMode === 'advanced' ? styles.modeBtnActive : ''}`}
              onClick={() => setMode('advanced')}
              title="Code Mode"
            >
              <Code size={14} />
            </button>
          </div>

          {/* Element picker toggle */}
          <button
            className={`${styles.toolBtn} ${isPickerActive ? styles.toolBtnActive : ''}`}
            onClick={togglePicker}
            title="Element Picker (or click any element on the page)"
          >
            <Crosshair size={15} />
          </button>

          {/* Element list dropdown */}
          <ElementListDropdown onSelectElement={onSelectElement} />

          <div className={styles.divider} />

          {/* Window controls */}
          <button className={styles.windowBtn} onClick={onMinimize} title="Minimize">
            <Minus size={15} />
          </button>
          <button className={`${styles.windowBtn} ${styles.closeBtn}`} onClick={onClose} title="Close">
            <X size={15} />
          </button>
        </div>
      </div>
    );
  }
);

PanelTitleBar.displayName = 'PanelTitleBar';
export default PanelTitleBar;
