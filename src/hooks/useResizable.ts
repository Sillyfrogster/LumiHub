import { useRef, useCallback, useEffect } from 'react';

interface UseResizableOptions {
  size: { width: number; height: number };
  onSizeChange: (size: { width: number; height: number }) => void;
  panelRef: React.RefObject<HTMLElement | null>;
  minWidth?: number;
  minHeight?: number;
  maxWidth?: number;
  maxHeight?: number;
}

type Edge = 'right' | 'bottom' | 'corner';

export function useResizable({
  size,
  onSizeChange,
  panelRef,
  minWidth = 360,
  minHeight = 400,
  maxWidth = window.innerWidth * 0.9,
  maxHeight = window.innerHeight * 0.9,
}: UseResizableOptions) {
  const isResizing = useRef(false);
  const activeEdge = useRef<Edge>('corner');
  const startPos = useRef({ x: 0, y: 0 });
  const startSize = useRef({ width: 0, height: 0 });
  const rafId = useRef<number>(0);
  const latestSize = useRef({ width: size.width, height: size.height });

  const startResize = useCallback((edge: Edge) => (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    isResizing.current = true;
    activeEdge.current = edge;

    const panel = panelRef.current;
    if (!panel) return;

    startPos.current = { x: e.clientX, y: e.clientY };
    startSize.current = { width: panel.offsetWidth, height: panel.offsetHeight };
    document.body.style.userSelect = 'none';
    document.body.style.cursor =
      edge === 'right' ? 'ew-resize' :
      edge === 'bottom' ? 'ns-resize' : 'nwse-resize';
  }, [panelRef]);

  const onMouseMove = useCallback((e: MouseEvent) => {
    if (!isResizing.current) return;
    const panel = panelRef.current;
    if (!panel) return;

    const dx = e.clientX - startPos.current.x;
    const dy = e.clientY - startPos.current.y;
    const edge = activeEdge.current;

    let newWidth = startSize.current.width;
    let newHeight = startSize.current.height;

    if (edge === 'right' || edge === 'corner') {
      newWidth = Math.max(minWidth, Math.min(startSize.current.width + dx, maxWidth));
    }
    if (edge === 'bottom' || edge === 'corner') {
      newHeight = Math.max(minHeight, Math.min(startSize.current.height + dy, maxHeight));
    }

    latestSize.current = { width: newWidth, height: newHeight };

    if (rafId.current) cancelAnimationFrame(rafId.current);
    rafId.current = requestAnimationFrame(() => {
      panel.style.width = `${latestSize.current.width}px`;
      panel.style.height = `${latestSize.current.height}px`;
    });
  }, [panelRef, minWidth, minHeight, maxWidth, maxHeight]);

  const onMouseUp = useCallback(() => {
    if (!isResizing.current) return;
    isResizing.current = false;
    document.body.style.userSelect = '';
    document.body.style.cursor = '';
    onSizeChange(latestSize.current);
  }, [onSizeChange]);

  useEffect(() => {
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
    return () => {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
      if (rafId.current) cancelAnimationFrame(rafId.current);
    };
  }, [onMouseMove, onMouseUp]);

  return { startResize };
}
