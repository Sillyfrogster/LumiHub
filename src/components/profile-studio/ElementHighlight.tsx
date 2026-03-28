import { generateSelector } from '../../utils/selectorGenerator';
import styles from './ElementHighlight.module.css';

interface ElementHighlightProps {
  hoveredElement: HTMLElement | null;
  hoveredRect: DOMRect | null;
  selectedElement: HTMLElement | null;
  selectedRect: DOMRect | null;
}

const ElementHighlight = ({
  hoveredElement,
  hoveredRect,
  selectedElement,
  selectedRect,
}: ElementHighlightProps) => {
  return (
    <>
      {/* Hover highlight (blue) */}
      {hoveredRect && hoveredElement && hoveredElement !== selectedElement && (
        <div
          className={styles.hoverOverlay}
          data-picker-overlay
          style={{
            left: hoveredRect.left,
            top: hoveredRect.top,
            width: hoveredRect.width,
            height: hoveredRect.height,
          }}
        >
          <div className={styles.tooltip}>
            {generateSelector(hoveredElement).friendlyName}
            <span className={styles.tooltipSelector}>
              {generateSelector(hoveredElement).selector}
            </span>
          </div>
        </div>
      )}

      {/* Selected highlight (pink) */}
      {selectedRect && selectedElement && (
        <div
          className={styles.selectedOverlay}
          data-picker-overlay
          style={{
            left: selectedRect.left,
            top: selectedRect.top,
            width: selectedRect.width,
            height: selectedRect.height,
          }}
        >
          <div className={styles.tooltipSelected}>
            {generateSelector(selectedElement).friendlyName}
          </div>
        </div>
      )}
    </>
  );
};

export default ElementHighlight;
