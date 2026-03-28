import { useState, useEffect } from 'react';
import styles from './Controls.module.css';

interface ColorPickerProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
}

/** Convert any CSS color to a hex value for the native input, best-effort. */
function toHex(color: string): string {
  if (/^#[0-9a-fA-F]{6}$/.test(color)) return color;
  if (/^#[0-9a-fA-F]{3}$/.test(color)) {
    return '#' + color[1] + color[1] + color[2] + color[2] + color[3] + color[3];
  }
  return '#000000';
}

const ColorPicker = ({ label, value, onChange }: ColorPickerProps) => {
  const [hex, setHex] = useState(toHex(value));
  const [text, setText] = useState(value || '');

  useEffect(() => {
    setText(value || '');
    setHex(toHex(value));
  }, [value]);

  return (
    <div className={styles.controlRow}>
      <label className={styles.label}>{label}</label>
      <div className={styles.colorGroup}>
        <input
          type="color"
          className={styles.colorInput}
          value={hex}
          onChange={(e) => {
            setHex(e.target.value);
            setText(e.target.value);
            onChange(e.target.value);
          }}
        />
        <input
          type="text"
          className={styles.textInput}
          value={text}
          placeholder="#000000"
          onChange={(e) => {
            setText(e.target.value);
          }}
          onBlur={() => {
            if (text.trim()) {
              onChange(text.trim());
              setHex(toHex(text.trim()));
            }
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              onChange(text.trim());
              setHex(toHex(text.trim()));
            }
          }}
        />
      </div>
    </div>
  );
};

export default ColorPicker;
