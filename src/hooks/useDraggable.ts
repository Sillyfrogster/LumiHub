import { useRef, useCallback, useEffect } from 'react';

interface UseDraggableOptions {
  position: { x: number; y: number };
  onPositionChange: (pos: { x: number; y: number }) => void;
  handleRef: React.RefObject<HTMLElement | null>;
  panelRef: React.RefObject<HTMLElement | null>;
}

export function useDraggable({ position, onPositionChange, handleRef, panelRef }: UseDraggableOptions) {
  const isDragging = useRef(false);
  const offset = useRef({ x: 0, y: 0 });
  const rafId = useRef<number>(0);
  const latestPos = useRef({ x: 0, y: 0 });

  const onMouseDown = useCallback((e: MouseEvent) => {
    if ((e.target as HTMLElement).closest('button')) return;

    isDragging.current = true;
    const panel = panelRef.current;
    if (!panel) return;

    const rect = panel.getBoundingClientRect();
    offset.current = { x: e.clientX - rect.left, y: e.clientY - rect.top };
    document.body.style.userSelect = 'none';
    document.body.style.cursor = 'grabbing';
  }, [panelRef]);

  const onMouseMove = useCallback((e: MouseEvent) => {
    if (!isDragging.current) return;

    const panel = panelRef.current;
    if (!panel) return;

    const maxX = window.innerWidth - panel.offsetWidth;
    const maxY = window.innerHeight - 40;

    const x = Math.max(0, Math.min(e.clientX - offset.current.x, maxX));
    const y = Math.max(0, Math.min(e.clientY - offset.current.y, maxY));

    latestPos.current = { x, y };

    if (rafId.current) cancelAnimationFrame(rafId.current);
    rafId.current = requestAnimationFrame(() => {
      panel.style.left = `${latestPos.current.x}px`;
      panel.style.top = `${latestPos.current.y}px`;
    });
  }, [panelRef]);

  const onMouseUp = useCallback(() => {
    if (!isDragging.current) return;
    isDragging.current = false;
    document.body.style.userSelect = '';
    document.body.style.cursor = '';
    onPositionChange(latestPos.current);
  }, [onPositionChange]);

  useEffect(() => {
    const handle = handleRef.current;
    if (!handle) return;

    handle.addEventListener('mousedown', onMouseDown);
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);

    return () => {
      handle.removeEventListener('mousedown', onMouseDown);
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
      if (rafId.current) cancelAnimationFrame(rafId.current);
    };
  }, [handleRef, onMouseDown, onMouseMove, onMouseUp]);

  const resolvedPosition = {
    x: position.x < 0 ? Math.max(0, window.innerWidth - 520) : position.x,
    y: position.y,
  };

  return { resolvedPosition };
}
