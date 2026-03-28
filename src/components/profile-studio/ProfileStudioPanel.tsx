import { useRef, useState, useCallback } from 'react';
import { useProfileStudioStore } from '../../store/useProfileStudioStore';
import { useDraggable } from '../../hooks/useDraggable';
import { useResizable } from '../../hooks/useResizable';
import { ProfileStudioProvider } from './ProfileStudioContext';
import PanelTitleBar from './PanelTitleBar';
import PanelTabBar from './PanelTabBar';
import PanelFooter from './PanelFooter';
import HtmlEditorTab from './HtmlEditorTab';
import CssEditorTab from './CssEditorTab';
import AssetsTab from './AssetsTab';
import styles from './ProfileStudioPanel.module.css';

interface ProfileStudioPanelProps {
  onSave: () => void;
  onReset: () => void;
  onSelectElement: (element: HTMLElement) => void;
}

const ProfileStudioPanel = ({ onSave, onReset, onSelectElement }: ProfileStudioPanelProps) => {
  const {
    activeTab,
    panelPosition,
    panelSize,
    isDirty,
    setPanelPosition,
    setPanelSize,
    close,
  } = useProfileStudioStore();

  const panelRef = useRef<HTMLDivElement>(null);
  const titleBarRef = useRef<HTMLDivElement>(null);
  const [isMinimized, setIsMinimized] = useState(false);

  const { resolvedPosition } = useDraggable({
    position: panelPosition,
    onPositionChange: setPanelPosition,
    handleRef: titleBarRef,
    panelRef,
  });

  const { startResize } = useResizable({
    size: panelSize,
    onSizeChange: setPanelSize,
    panelRef,
  });

  const handleClose = useCallback(() => {
    if (isDirty && !window.confirm('You have unsaved changes. Discard them?')) {
      return;
    }
    close();
  }, [isDirty, close]);

  const handleMinimize = useCallback(() => {
    setIsMinimized((v) => !v);
  }, []);

  return (
    <ProfileStudioProvider>
      <div
        ref={panelRef}
        className={`${styles.panel} ${isMinimized ? styles.minimized : ''}`}
        style={{
          left: resolvedPosition.x,
          top: resolvedPosition.y,
          width: isMinimized ? 240 : panelSize.width,
          height: isMinimized ? 'auto' : panelSize.height,
        }}
        data-studio-panel
      >
        <PanelTitleBar
          ref={titleBarRef}
          onClose={handleClose}
          onMinimize={handleMinimize}
          onSelectElement={onSelectElement}
        />

        {!isMinimized && (
          <>
            <PanelTabBar />

            <div className={styles.content}>
              {activeTab === 'html' && <HtmlEditorTab />}
              {activeTab === 'css' && <CssEditorTab />}
              {activeTab === 'assets' && <AssetsTab />}
            </div>

            <PanelFooter onSave={onSave} onReset={onReset} />

            {/* Resize handles */}
            <div
              className={`${styles.resizeHandle} ${styles.resizeRight}`}
              onMouseDown={startResize('right')}
            />
            <div
              className={`${styles.resizeHandle} ${styles.resizeBottom}`}
              onMouseDown={startResize('bottom')}
            />
            <div
              className={`${styles.resizeHandle} ${styles.resizeCorner}`}
              onMouseDown={startResize('corner')}
            />
          </>
        )}
      </div>
    </ProfileStudioProvider>
  );
};

export default ProfileStudioPanel;
