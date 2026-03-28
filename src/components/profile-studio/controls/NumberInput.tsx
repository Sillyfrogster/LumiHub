import { useState, useEffect } from 'react';
import { ChevronUp, ChevronDown } from 'lucide-react';
import styles from './Controls.module.css';

interface NumberInputProps {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
}

const NumberInput = ({ label, value, onChange, min = 0, max = 999, step = 1, unit = 'px' }: NumberInputProps) => {
  const [text, setText] = useState(String(value));

  useEffect(() => {
    setText(String(value));
  }, [value]);

  const clamp = (n: number) => Math.max(min, Math.min(max, n));

  const commit = (raw: string) => {
    const n = parseFloat(raw);
    if (!isNaN(n)) onChange(clamp(n));
  };

  return (
    <div className={styles.controlRow}>
      <label className={styles.label}>{label}</label>
      <div className={styles.numberGroup}>
        <input
          type="text"
          className={styles.numberField}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onBlur={() => commit(text)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') commit(text);
            if (e.key === 'ArrowUp') { e.preventDefault(); onChange(clamp(value + step)); }
            if (e.key === 'ArrowDown') { e.preventDefault(); onChange(clamp(value - step)); }
          }}
        />
        <div className={styles.numberBtnGroup}>
          <button className={styles.numberBtn} onClick={() => onChange(clamp(value + step))}>
            <ChevronUp size={10} />
          </button>
          <button className={styles.numberBtn} onClick={() => onChange(clamp(value - step))}>
            <ChevronDown size={10} />
          </button>
        </div>
      </div>
      {unit && <span className={styles.unitLabel}>{unit}</span>}
    </div>
  );
};

export default NumberInput;
