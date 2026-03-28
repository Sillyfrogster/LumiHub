import { useState, useEffect, useRef, useCallback } from 'react';
import { ChevronDown, MousePointer2 } from 'lucide-react';
import { getElementName } from '../../utils/elementNames';
import { generateSelector } from '../../utils/selectorGenerator';
import styles from './ElementListDropdown.module.css';

interface ElementEntry {
  element: HTMLElement;
  studioValue: string;
  friendlyName: string;
  selector: string;
  depth: number;
}

interface ElementListDropdownProps {
  onSelectElement: (element: HTMLElement) => void;
}

const ElementListDropdown = ({ onSelectElement }: ElementListDropdownProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const [entries, setEntries] = useState<ElementEntry[]>([]);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const scanElements = useCallback(() => {
    const elements = document.querySelectorAll<HTMLElement>('[data-studio]');
    const scanned: ElementEntry[] = [];

    elements.forEach((el) => {
      if (el.closest('[data-studio-panel]')) return;

      const studioValue = el.getAttribute('data-studio') || '';
      const { selector } = generateSelector(el);

      let depth = 0;
      let parent = el.parentElement;
      while (parent) {
        if (parent.hasAttribute('data-studio') && !parent.closest('[data-studio-panel]')) {
          depth++;
        }
        parent = parent.parentElement;
      }

      scanned.push({
        element: el,
        studioValue,
        friendlyName: getElementName(studioValue),
        selector,
        depth,
      });
    });

    setEntries(scanned);
  }, []);

  useEffect(() => {
    if (isOpen) scanElements();
  }, [isOpen, scanElements]);

  useEffect(() => {
    if (!isOpen) return;
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [isOpen]);

  const handleSelect = (entry: ElementEntry) => {
    entry.element.scrollIntoView({ behavior: 'smooth', block: 'center' });
    onSelectElement(entry.element);
    setIsOpen(false);
  };

  return (
    <div className={styles.wrapper} ref={dropdownRef}>
      <button
        className={`${styles.trigger} ${isOpen ? styles.triggerActive : ''}`}
        onClick={() => setIsOpen((v) => !v)}
        title="Element List"
      >
        <ChevronDown size={15} />
      </button>

      {isOpen && (
        <div className={styles.dropdown}>
          <div className={styles.header}>
            <MousePointer2 size={12} />
            <span>Page Elements</span>
          </div>
          <div className={styles.list}>
            {entries.map((entry, i) => (
              <button
                key={`${entry.studioValue}-${i}`}
                className={styles.item}
                style={{ paddingLeft: 12 + entry.depth * 16 }}
                onClick={() => handleSelect(entry)}
              >
                <span className={styles.itemName}>{entry.friendlyName}</span>
                <span className={styles.itemSelector}>{entry.selector}</span>
              </button>
            ))}
            {entries.length === 0 && (
              <div className={styles.empty}>No elements found</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default ElementListDropdown;
