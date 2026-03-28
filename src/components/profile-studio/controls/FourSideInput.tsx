import { useState, useEffect } from 'react';
import styles from './Controls.module.css';

interface FourSideValues {
  top: string;
  right: string;
  bottom: string;
  left: string;
}

interface FourSideInputProps {
  label: string;
  value: FourSideValues;
  onChange: (value: FourSideValues) => void;
  unit?: string;
}

export function parseShorthand(shorthand: string): FourSideValues {
  const parts = shorthand.replace(/px|em|rem|%/g, '').trim().split(/\s+/);
  if (parts.length === 1) return { top: parts[0], right: parts[0], bottom: parts[0], left: parts[0] };
  if (parts.length === 2) return { top: parts[0], right: parts[1], bottom: parts[0], left: parts[1] };
  if (parts.length === 3) return { top: parts[0], right: parts[1], bottom: parts[2], left: parts[1] };
  return { top: parts[0] || '0', right: parts[1] || '0', bottom: parts[2] || '0', left: parts[3] || '0' };
}

export function toShorthand(values: FourSideValues, unit: string = 'px'): string {
  const { top, right, bottom, left } = values;
  const t = top || '0', r = right || '0', b = bottom || '0', l = left || '0';
  if (t === r && r === b && b === l) return `${t}${unit}`;
  if (t === b && r === l) return `${t}${unit} ${r}${unit}`;
  if (r === l) return `${t}${unit} ${r}${unit} ${b}${unit}`;
  return `${t}${unit} ${r}${unit} ${b}${unit} ${l}${unit}`;
}

const FourSideInput = ({ label, value, onChange, unit = 'px' }: FourSideInputProps) => {
  const [local, setLocal] = useState(value);

  useEffect(() => {
    setLocal(value);
  }, [value.top, value.right, value.bottom, value.left]);

  const update = (side: keyof FourSideValues, raw: string) => {
    const next = { ...local, [side]: raw };
    setLocal(next);
    onChange(next);
  };

  return (
    <div className={styles.controlRow} style={{ alignItems: 'flex-start' }}>
      <label className={styles.label} style={{ paddingTop: 4 }}>{label}</label>
      <div className={styles.fourSideWrapper}>
        <div className={styles.fourSideRow}>
          <span className={styles.fourSideLabel}>T</span>
          <input
            type="text"
            className={styles.fourSideField}
            value={local.top}
            onChange={(e) => update('top', e.target.value)}
            placeholder="0"
          />
          <span className={styles.fourSideLabel}>R</span>
          <input
            type="text"
            className={styles.fourSideField}
            value={local.right}
            onChange={(e) => update('right', e.target.value)}
            placeholder="0"
          />
        </div>
        <div className={styles.fourSideRow}>
          <span className={styles.fourSideLabel}>B</span>
          <input
            type="text"
            className={styles.fourSideField}
            value={local.bottom}
            onChange={(e) => update('bottom', e.target.value)}
            placeholder="0"
          />
          <span className={styles.fourSideLabel}>L</span>
          <input
            type="text"
            className={styles.fourSideField}
            value={local.left}
            onChange={(e) => update('left', e.target.value)}
            placeholder="0"
          />
        </div>
        {unit && <span className={styles.unitLabel} style={{ alignSelf: 'flex-end' }}>{unit}</span>}
      </div>
    </div>
  );
};

export default FourSideInput;
