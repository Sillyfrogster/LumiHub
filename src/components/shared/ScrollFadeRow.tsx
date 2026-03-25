import { useRef, useState, useEffect, useCallback, type ReactNode } from 'react';
import styles from './ScrollFadeRow.module.css';

interface Props {
  children: ReactNode;
  className?: string;
  as?: 'div' | 'nav';
  /** ARIA role for the inner scrollable element. */
  role?: string;
  /** ARIA label for the inner scrollable element. */
  'aria-label'?: string;
}

/**
 * Wraps a horizontally-scrollable row with fade gradients on the
 * left/right edges to indicate overflow. Gradients appear dynamically
 * based on scroll position.
 */
const ScrollFadeRow: React.FC<Props> = ({ children, className, as: Tag = 'div', role, ...rest }) => {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  const check = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setCanScrollLeft(el.scrollLeft > 2);
    setCanScrollRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 2);
  }, []);

  useEffect(() => {
    check();
    const el = scrollRef.current;
    if (!el) return;
    const ro = new ResizeObserver(check);
    ro.observe(el);
    return () => ro.disconnect();
  }, [check, children]);

  const wrapCls = [
    styles.wrap,
    canScrollLeft ? styles.fadeLeft : '',
    canScrollRight ? styles.fadeRight : '',
  ].filter(Boolean).join(' ');

  return (
    <Tag className={wrapCls}>
      <div
        ref={scrollRef}
        className={`${styles.scroll} ${className || ''}`}
        onScroll={check}
        role={role}
        aria-label={rest['aria-label']}
      >
        {children}
      </div>
    </Tag>
  );
};

export default ScrollFadeRow;
