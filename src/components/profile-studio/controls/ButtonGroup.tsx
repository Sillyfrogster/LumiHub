import type { ReactNode } from 'react';
import styles from './Controls.module.css';

interface ButtonGroupOption {
  value: string;
  label: ReactNode;
}

interface ButtonGroupProps {
  label: string;
  value: string;
  options: ButtonGroupOption[];
  onChange: (value: string) => void;
}

const ButtonGroup = ({ label, value, options, onChange }: ButtonGroupProps) => {
  return (
    <div className={styles.controlRow}>
      <label className={styles.label}>{label}</label>
      <div className={styles.buttonGroup}>
        {options.map((opt) => (
          <button
            key={opt.value}
            className={`${styles.groupBtn} ${value === opt.value ? styles.groupBtnActive : ''}`}
            onClick={() => onChange(opt.value)}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );
};

export default ButtonGroup;
