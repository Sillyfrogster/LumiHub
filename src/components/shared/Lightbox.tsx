import { useState, useCallback, useRef, useEffect, type PointerEvent as ReactPointerEvent } from 'react';
import { X, ChevronLeft, ChevronRight, ZoomIn, ZoomOut, RotateCcw } from 'lucide-react';
import styles from './Lightbox.module.css';

export interface LightboxImage {
  src: string;
  alt?: string;
}

interface LightboxProps {
  images: LightboxImage[];
  startIndex?: number;
  onClose: () => void;
}

const MIN_SCALE = 1;
const MAX_SCALE = 5;
const ZOOM_STEP = 0.5;

export default function Lightbox({ images, startIndex = 0, onClose }: LightboxProps) {
  const [index, setIndex] = useState(startIndex);
  const [scale, setScale] = useState(1);
  const [translate, setTranslate] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const dragStart = useRef({ x: 0, y: 0 });
  const translateStart = useRef({ x: 0, y: 0 });
  const imgRef = useRef<HTMLDivElement>(null);

  const current = images[index];
  const hasMultiple = images.length > 1;

  // Reset zoom/pan when navigating
  const resetView = useCallback(() => {
    setScale(1);
    setTranslate({ x: 0, y: 0 });
  }, []);

  const goTo = useCallback((i: number) => {
    setIndex(i);
    resetView();
  }, [resetView]);

  const goPrev = useCallback(() => {
    if (hasMultiple) goTo((index - 1 + images.length) % images.length);
  }, [index, images.length, hasMultiple, goTo]);

  const goNext = useCallback(() => {
    if (hasMultiple) goTo((index + 1) % images.length);
  }, [index, images.length, hasMultiple, goTo]);

  const zoomIn = useCallback(() => {
    setScale((s) => Math.min(s + ZOOM_STEP, MAX_SCALE));
  }, []);

  const zoomOut = useCallback(() => {
    setScale((s) => {
      const next = Math.max(s - ZOOM_STEP, MIN_SCALE);
      if (next === MIN_SCALE) setTranslate({ x: 0, y: 0 });
      return next;
    });
  }, []);

  // Keyboard navigation
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'Escape': onClose(); break;
        case 'ArrowLeft': goPrev(); break;
        case 'ArrowRight': goNext(); break;
        case '+': case '=': zoomIn(); break;
        case '-': zoomOut(); break;
        case '0': resetView(); break;
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onClose, goPrev, goNext, zoomIn, zoomOut, resetView]);

  // Prevent body scroll while open
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => { document.body.style.overflow = prev; };
  }, []);

  // Wheel zoom
  const handleWheel = useCallback((e: React.WheelEvent) => {
    e.preventDefault();
    if (e.deltaY < 0) zoomIn();
    else zoomOut();
  }, [zoomIn, zoomOut]);

  // Pointer drag for panning
  const handlePointerDown = useCallback((e: ReactPointerEvent) => {
    if (scale <= 1) return;
    e.preventDefault();
    setIsDragging(true);
    dragStart.current = { x: e.clientX, y: e.clientY };
    translateStart.current = { ...translate };
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
  }, [scale, translate]);

  const handlePointerMove = useCallback((e: ReactPointerEvent) => {
    if (!isDragging) return;
    const dx = e.clientX - dragStart.current.x;
    const dy = e.clientY - dragStart.current.y;
    setTranslate({
      x: translateStart.current.x + dx,
      y: translateStart.current.y + dy,
    });
  }, [isDragging]);

  const handlePointerUp = useCallback(() => {
    setIsDragging(false);
  }, []);

  // Click backdrop to close (only if not zoomed/panning)
  const handleBackdropClick = useCallback((e: React.MouseEvent) => {
    if (e.target === e.currentTarget && scale <= 1) onClose();
  }, [onClose, scale]);

  // Double-click to toggle zoom
  const handleDoubleClick = useCallback(() => {
    if (scale > 1) {
      resetView();
    } else {
      setScale(2.5);
    }
  }, [scale, resetView]);

  if (!current) return null;

  return (
    <div className={styles.overlay} onClick={handleBackdropClick} role="dialog" aria-modal="true" aria-label="Image viewer">
      {/* Top toolbar */}
      <div className={styles.toolbar}>
        {hasMultiple && (
          <span className={styles.counter}>{index + 1} / {images.length}</span>
        )}
        <div className={styles.toolbarActions}>
          <button className={styles.toolBtn} onClick={zoomIn} title="Zoom in (+)" aria-label="Zoom in">
            <ZoomIn size={18} />
          </button>
          <button className={styles.toolBtn} onClick={zoomOut} title="Zoom out (-)" aria-label="Zoom out">
            <ZoomOut size={18} />
          </button>
          {scale > 1 && (
            <button className={styles.toolBtn} onClick={resetView} title="Reset zoom (0)" aria-label="Reset zoom">
              <RotateCcw size={18} />
            </button>
          )}
          <button className={styles.closeBtn} onClick={onClose} title="Close (Esc)" aria-label="Close">
            <X size={20} />
          </button>
        </div>
      </div>

      {/* Navigation arrows */}
      {hasMultiple && (
        <>
          <button className={`${styles.navBtn} ${styles.navPrev}`} onClick={goPrev} aria-label="Previous image">
            <ChevronLeft size={28} />
          </button>
          <button className={`${styles.navBtn} ${styles.navNext}`} onClick={goNext} aria-label="Next image">
            <ChevronRight size={28} />
          </button>
        </>
      )}

      {/* Image viewport */}
      <div
        className={styles.viewport}
        ref={imgRef}
        onWheel={handleWheel}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onDoubleClick={handleDoubleClick}
        style={{ cursor: scale > 1 ? (isDragging ? 'grabbing' : 'grab') : 'default' }}
      >
        <img
          key={current.src}
          src={current.src}
          alt={current.alt || ''}
          className={styles.image}
          style={{
            transform: `translate(${translate.x}px, ${translate.y}px) scale(${scale})`,
            transition: isDragging ? 'none' : 'transform 0.2s ease',
          }}
          draggable={false}
        />
      </div>
    </div>
  );
}
