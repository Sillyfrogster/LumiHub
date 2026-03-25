import React from 'react';
import styles from './FilterSidebar.module.css';

interface FilterSectionProps {
  label: string;
  children: React.ReactNode;
}

export const FilterSection: React.FC<FilterSectionProps> = ({ label, children }) => (
  <div className={styles.filterSection}>
    <h3 className={styles.filterLabel}>{label}</h3>
    {children}
  </div>
);

export const FilterRadioGroup: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className={styles.radioGroup}>
    {children}
  </div>
);

interface FilterRadioOptionProps {
  name: string;
  value: string;
  label: string;
  checked: boolean;
  onChange: (val: string) => void;
  icon?: React.ReactNode;
}

export const FilterRadioOption: React.FC<FilterRadioOptionProps> = ({ 
  name, value, label, checked, onChange, icon 
}) => (
  <label className={`${styles.radioOption} ${checked ? styles.radioActive : ''}`}>
    <input
      type="radio"
      name={name}
      value={value}
      checked={checked}
      onChange={() => onChange(value)}
      className={styles.radioInput}
    />
    {icon && <span className={styles.filterIcon}>{icon}</span>}
    <span className={styles.filterText}>{label}</span>
    {checked && <span className={styles.activeDot} />}
  </label>
);

export const FilterSortList: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className={styles.sortList}>
    {children}
  </div>
);

interface FilterSortOptionProps {
  label: string;
  active: boolean;
  onClick: () => void;
  icon?: React.ReactNode;
}

export const FilterSortOption: React.FC<FilterSortOptionProps> = ({ 
  label, active, onClick, icon 
}) => (
  <button
    className={`${styles.sortOption} ${active ? styles.sortActive : ''}`}
    onClick={onClick}
  >
    {icon && <span className={styles.filterIcon}>{icon}</span>}
    <span className={styles.filterText}>{label}</span>
    {active && <span className={styles.activeDot} />}
  </button>
);

interface FilterCheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}

export const FilterCheckbox: React.FC<FilterCheckboxProps> = ({ label, checked, onChange, disabled }) => (
  <label className={`${styles.checkboxOption} ${disabled ? styles.checkboxDisabled : ''}`}>
    <input
      type="checkbox"
      checked={checked}
      onChange={(e) => onChange(e.target.checked)}
      disabled={disabled}
    />
    <span>{label}</span>
  </label>
);

interface FilterNumberInputProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  suffix?: string;
  placeholder?: string;
}

export const FilterNumberInput: React.FC<FilterNumberInputProps> = ({
  value, onChange, min = 0, max, step = 1, suffix, placeholder,
}) => (
  <div className={styles.numberInputWrap}>
    <input
      type="number"
      className={styles.numberInput}
      value={value}
      onChange={(e) => onChange(Number(e.target.value))}
      min={min}
      max={max}
      step={step}
      placeholder={placeholder}
    />
    {suffix && <span className={styles.numberInputSuffix}>{suffix}</span>}
  </div>
);

export const FilterPlaceholder: React.FC<{ text: string }> = ({ text }) => (
  <p className={styles.filterHint}>{text}</p>
);

interface FilterTagInputProps {
  tags: string[];
  onAdd: (tag: string) => void;
  onRemove: (tag: string) => void;
  availableTags: { name: string; count: number }[];
  loading?: boolean;
  onSearchChange: (search: string) => void;
  placeholder?: string;
  variant?: 'include' | 'exclude';
}

function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, '')}m`;
  if (n >= 10_000) return `${Math.round(n / 1000)}k`;
  if (n >= 1_000) return `${(n / 1000).toFixed(1).replace(/\.0$/, '')}k`;
  return String(n);
}

export const FilterTagInput: React.FC<FilterTagInputProps> = ({
  tags,
  onAdd,
  onRemove,
  availableTags,
  loading = false,
  onSearchChange,
  placeholder = 'Search tags…',
  variant = 'include',
}) => {
  const [value, setValue] = React.useState('');
  const [open, setOpen] = React.useState(false);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const wrapRef = React.useRef<HTMLDivElement>(null);

  const isExclude = variant === 'exclude';
  const selectedSet = React.useMemo(() => new Set(tags.map((t) => t.toLowerCase())), [tags]);

  // Close on outside click
  React.useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  // Propagate search changes (debounced by caller or immediate)
  React.useEffect(() => {
    const timer = setTimeout(() => onSearchChange(value), 250);
    return () => clearTimeout(timer);
  }, [value, onSearchChange]);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Escape') {
      setOpen(false);
      inputRef.current?.blur();
    }
  };

  const toggleTag = (tagName: string) => {
    const normalized = tagName.toLowerCase();
    if (selectedSet.has(normalized)) {
      onRemove(normalized);
    } else {
      onAdd(normalized);
    }
    setValue('');
    onSearchChange('');
    inputRef.current?.focus();
  };

  return (
    <div className={styles.tagCombobox} ref={wrapRef}>
      {/* Selected chips */}
      {tags.length > 0 && (
        <div className={styles.tagChips}>
          {tags.map((tag) => (
            <span
              key={tag}
              className={`${styles.tagChip} ${isExclude ? styles.tagChipExclude : ''}`}
            >
              <span className={styles.tagChipLabel}>{tag}</span>
              <button
                type="button"
                className={styles.tagChipRemove}
                onClick={() => onRemove(tag)}
                aria-label={`Remove ${tag}`}
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}

      {/* Search input */}
      <div className={`${styles.tagInputWrap} ${open ? styles.tagInputWrapOpen : ''}`}>
        <input
          ref={inputRef}
          type="text"
          className={styles.tagInput}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onFocus={() => setOpen(true)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          spellCheck={false}
          autoComplete="off"
        />
        {loading && <span className={styles.tagSpinner} />}
      </div>

      {/* Dropdown */}
      {open && (
        <div className={styles.tagDropdown}>
          {loading && availableTags.length === 0 ? (
            <div className={styles.tagDropdownEmpty}>Loading tags…</div>
          ) : availableTags.length === 0 ? (
            <div className={styles.tagDropdownEmpty}>No tags found</div>
          ) : (
            <div className={styles.tagDropdownList}>
              {availableTags.map((t) => {
                const isSelected = selectedSet.has(t.name.toLowerCase());
                return (
                  <button
                    key={t.name}
                    type="button"
                    className={`${styles.tagOption} ${isSelected ? (isExclude ? styles.tagOptionSelectedExclude : styles.tagOptionSelected) : ''}`}
                    onMouseDown={(e) => {
                      e.preventDefault(); // prevent blur
                      toggleTag(t.name);
                    }}
                  >
                    <span className={styles.tagOptionCheck}>
                      {isSelected ? '✓' : ''}
                    </span>
                    <span className={styles.tagOptionName}>{t.name}</span>
                    {t.count > 0 && (
                      <span className={styles.tagOptionCount}>
                        {formatCount(t.count)}
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export const FilterSidebar: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <>{children}</>
);
