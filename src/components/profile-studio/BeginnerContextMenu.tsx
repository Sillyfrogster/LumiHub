import { useState, useCallback, useMemo, useEffect } from 'react';
import { ChevronDown, ChevronRight, X } from 'lucide-react';
import { useProfileStudioStore } from '../../store/useProfileStudioStore';
import { generateSelector } from '../../utils/selectorGenerator';
import { upsertCssRule, removeCssProperty, getCssPropertyValue } from '../../utils/cssGenerator';
import { parseShorthand, toShorthand } from './controls/FourSideInput';
import ColorPicker from './controls/ColorPicker';
import NumberInput from './controls/NumberInput';
import SelectInput from './controls/SelectInput';
import ButtonGroup from './controls/ButtonGroup';
import FourSideInput from './controls/FourSideInput';
import styles from './BeginnerContextMenu.module.css';

interface BeginnerContextMenuProps {
  element: HTMLElement;
  rect: DOMRect;
  onClose: () => void;
}

type Section = 'background' | 'text' | 'spacing' | 'borders' | 'size' | 'effects';

const FONT_SIZE_OPTIONS = [
  { value: '12px', label: '12px' },
  { value: '14px', label: '14px' },
  { value: '16px', label: '16px' },
  { value: '18px', label: '18px' },
  { value: '20px', label: '20px' },
  { value: '24px', label: '24px' },
  { value: '28px', label: '28px' },
  { value: '32px', label: '32px' },
  { value: '36px', label: '36px' },
  { value: '40px', label: '40px' },
  { value: '48px', label: '48px' },
];

const BORDER_STYLE_OPTIONS = [
  { value: 'none', label: 'None' },
  { value: 'solid', label: 'Solid' },
  { value: 'dashed', label: 'Dashed' },
  { value: 'dotted', label: 'Dotted' },
];

const ALIGN_OPTIONS = [
  { value: 'left', label: 'L' },
  { value: 'center', label: 'C' },
  { value: 'right', label: 'R' },
];

const BOLD_OPTIONS = [
  { value: 'normal', label: 'Normal' },
  { value: 'bold', label: 'Bold' },
];

const BeginnerContextMenu = ({ element, rect, onClose }: BeginnerContextMenuProps) => {
  const { draftCss, setDraftCss } = useProfileStudioStore();
  const [openSections, setOpenSections] = useState<Set<Section>>(new Set(['background', 'text']));

  const { selector, friendlyName } = useMemo(() => generateSelector(element), [element]);

  // Read current CSS values for this selector
  const get = useCallback(
    (property: string) => getCssPropertyValue(draftCss, selector, property) || '',
    [draftCss, selector]
  );

  const set = useCallback(
    (property: string, value: string) => {
      if (!value || value === 'none' || value === '0px' || value === '0') {
        setDraftCss(removeCssProperty(draftCss, selector, property));
      } else {
        setDraftCss(upsertCssRule(draftCss, selector, property, value));
      }
    },
    [draftCss, selector, setDraftCss]
  );

  const toggleSection = (section: Section) => {
    setOpenSections((prev) => {
      const next = new Set(prev);
      if (next.has(section)) next.delete(section);
      else next.add(section);
      return next;
    });
  };

  const menuStyle = useMemo(() => {
    const menuWidth = 280;
    const menuMaxHeight = 400;
    let x = rect.right + 12;
    let y = rect.top;

    if (x + menuWidth > window.innerWidth - 20) {
      x = rect.left - menuWidth - 12;
    }
    if (x < 20) {
      x = Math.max(20, rect.left + rect.width / 2 - menuWidth / 2);
      y = rect.bottom + 12;
    }
    y = Math.max(20, Math.min(y, window.innerHeight - menuMaxHeight - 20));

    return { left: x, top: y };
  }, [rect]);

  useEffect(() => {
    setOpenSections(new Set(['background', 'text']));
  }, [element]);

  const renderSectionHeader = (section: Section, label: string) => {
    const isOpen = openSections.has(section);
    return (
      <button className={styles.sectionHeader} onClick={() => toggleSection(section)}>
        {isOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        <span>{label}</span>
      </button>
    );
  };

  return (
    <div className={styles.menu} style={menuStyle} data-picker-overlay>
      <div className={styles.header}>
        <div className={styles.headerInfo}>
          <span className={styles.elementName}>{friendlyName}</span>
          <span className={styles.selectorName}>{selector}</span>
        </div>
        <button className={styles.closeBtn} onClick={onClose} title="Close">
          <X size={14} />
        </button>
      </div>

      <div className={styles.body}>
        {/* Background */}
        {renderSectionHeader('background', 'Background')}
        {openSections.has('background') && (
          <div className={styles.sectionBody}>
            <ColorPicker label="Color" value={get('background-color')} onChange={(v) => set('background-color', v)} />
            <NumberInput
              label="Opacity"
              value={parseFloat(get('opacity') || '1')}
              onChange={(v) => set('opacity', String(v))}
              min={0}
              max={1}
              step={0.05}
              unit=""
            />
          </div>
        )}

        {/* Text */}
        {renderSectionHeader('text', 'Text')}
        {openSections.has('text') && (
          <div className={styles.sectionBody}>
            <ColorPicker label="Color" value={get('color')} onChange={(v) => set('color', v)} />
            <SelectInput label="Size" value={get('font-size') || '16px'} options={FONT_SIZE_OPTIONS} onChange={(v) => set('font-size', v)} />
            <ButtonGroup label="Weight" value={get('font-weight') || 'normal'} options={BOLD_OPTIONS} onChange={(v) => set('font-weight', v)} />
            <ButtonGroup label="Align" value={get('text-align') || 'left'} options={ALIGN_OPTIONS} onChange={(v) => set('text-align', v)} />
          </div>
        )}

        {/* Spacing */}
        {renderSectionHeader('spacing', 'Spacing')}
        {openSections.has('spacing') && (
          <div className={styles.sectionBody}>
            <FourSideInput
              label="Padding"
              value={parseShorthand(get('padding') || '0')}
              onChange={(v) => set('padding', toShorthand(v))}
            />
            <FourSideInput
              label="Margin"
              value={parseShorthand(get('margin') || '0')}
              onChange={(v) => set('margin', toShorthand(v))}
            />
          </div>
        )}

        {/* Borders */}
        {renderSectionHeader('borders', 'Borders')}
        {openSections.has('borders') && (
          <div className={styles.sectionBody}>
            <ColorPicker label="Color" value={get('border-color')} onChange={(v) => set('border-color', v)} />
            <NumberInput
              label="Width"
              value={parseInt(get('border-width') || '0', 10)}
              onChange={(v) => set('border-width', `${v}px`)}
              min={0}
              max={20}
            />
            <SelectInput label="Style" value={get('border-style') || 'none'} options={BORDER_STYLE_OPTIONS} onChange={(v) => set('border-style', v)} />
            <NumberInput
              label="Radius"
              value={parseInt(get('border-radius') || '0', 10)}
              onChange={(v) => set('border-radius', `${v}px`)}
              min={0}
              max={100}
            />
          </div>
        )}

        {/* Size */}
        {renderSectionHeader('size', 'Size')}
        {openSections.has('size') && (
          <div className={styles.sectionBody}>
            <SelectInput
              label="Width"
              value={get('width') || 'auto'}
              options={[
                { value: 'auto', label: 'Auto' },
                { value: '100%', label: '100%' },
                { value: '50%', label: '50%' },
                { value: '200px', label: '200px' },
                { value: '300px', label: '300px' },
                { value: '400px', label: '400px' },
              ]}
              onChange={(v) => set('width', v)}
            />
            <SelectInput
              label="Height"
              value={get('height') || 'auto'}
              options={[
                { value: 'auto', label: 'Auto' },
                { value: '100%', label: '100%' },
                { value: '50%', label: '50%' },
                { value: '100px', label: '100px' },
                { value: '200px', label: '200px' },
                { value: '300px', label: '300px' },
              ]}
              onChange={(v) => set('height', v)}
            />
          </div>
        )}

        {/* Effects */}
        {renderSectionHeader('effects', 'Effects')}
        {openSections.has('effects') && (
          <div className={styles.sectionBody}>
            <NumberInput
              label="Blur"
              value={parseInt(get('backdrop-filter')?.replace(/blur\((\d+)px\)/, '$1') || '0', 10)}
              onChange={(v) => set('backdrop-filter', v > 0 ? `blur(${v}px)` : '')}
              min={0}
              max={50}
            />
            <NumberInput
              label="Shadow X"
              value={parseShadowPart(get('box-shadow'), 0)}
              onChange={(v) => updateShadow(get, set, 0, v)}
              min={-50}
              max={50}
            />
            <NumberInput
              label="Shadow Y"
              value={parseShadowPart(get('box-shadow'), 1)}
              onChange={(v) => updateShadow(get, set, 1, v)}
              min={-50}
              max={50}
            />
            <NumberInput
              label="Sh. Blur"
              value={parseShadowPart(get('box-shadow'), 2)}
              onChange={(v) => updateShadow(get, set, 2, v)}
              min={0}
              max={100}
            />
            <ColorPicker
              label="Sh. Color"
              value={parseShadowColor(get('box-shadow'))}
              onChange={(v) => updateShadowColor(get, set, v)}
            />
          </div>
        )}
      </div>
    </div>
  );
};

function parseShadowPart(shadow: string, index: number): number {
  if (!shadow || shadow === 'none') return 0;
  const parts = shadow.match(/-?\d+px/g);
  if (!parts || !parts[index]) return 0;
  return parseInt(parts[index], 10);
}

function parseShadowColor(shadow: string): string {
  if (!shadow || shadow === 'none') return '#000000';
  const colorMatch = shadow.match(/(#[0-9a-fA-F]{3,8}|rgba?\([^)]+\))/);
  return colorMatch ? colorMatch[1] : '#000000';
}

function updateShadow(
  get: (prop: string) => string,
  set: (prop: string, val: string) => void,
  partIndex: number,
  value: number
) {
  const shadow = get('box-shadow');
  const parts = [
    parseShadowPart(shadow, 0),
    parseShadowPart(shadow, 1),
    parseShadowPart(shadow, 2),
  ];
  parts[partIndex] = value;
  const color = parseShadowColor(shadow);

  if (parts.every((p) => p === 0)) {
    set('box-shadow', '');
  } else {
    set('box-shadow', `${parts[0]}px ${parts[1]}px ${parts[2]}px ${color}`);
  }
}

function updateShadowColor(
  get: (prop: string) => string,
  set: (prop: string, val: string) => void,
  color: string
) {
  const shadow = get('box-shadow');
  const parts = [
    parseShadowPart(shadow, 0),
    parseShadowPart(shadow, 1),
    parseShadowPart(shadow, 2),
  ];
  set('box-shadow', `${parts[0]}px ${parts[1]}px ${parts[2]}px ${color}`);
}

export default BeginnerContextMenu;
