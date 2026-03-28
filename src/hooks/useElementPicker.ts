import { useState, useEffect, useCallback, useRef } from 'react';

interface PickerState {
  hoveredElement: HTMLElement | null;
  hoveredRect: DOMRect | null;
  selectedElement: HTMLElement | null;
  selectedRect: DOMRect | null;
}

interface UseElementPickerOptions {
  isActive: boolean;
  onDeactivate: () => void;
  onSelect: (element: HTMLElement) => void;
}

export function useElementPicker({ isActive, onDeactivate, onSelect }: UseElementPickerOptions) {
  const [state, setState] = useState<PickerState>({
    hoveredElement: null,
    hoveredRect: null,
    selectedElement: null,
    selectedRect: null,
  });

  const stateRef = useRef(state);
  stateRef.current = state;

  useEffect(() => {
    if (!isActive) {
      setState((s) => ({
        ...s,
        hoveredElement: null,
        hoveredRect: null,
      }));
    }
  }, [isActive]);

  useEffect(() => {
    if (isActive) {
      document.body.style.cursor = 'crosshair';
    } else {
      document.body.style.cursor = '';
    }
    return () => {
      document.body.style.cursor = '';
    };
  }, [isActive]);

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!isActive) return;

      const el = document.elementFromPoint(e.clientX, e.clientY) as HTMLElement | null;
      if (!el) return;
      if (el.closest('[data-studio-panel]')) {
        setState((s) => ({ ...s, hoveredElement: null, hoveredRect: null }));
        return;
      }

      if (el.closest('[data-picker-overlay]')) {
        return;
      }

      if (el !== stateRef.current.hoveredElement) {
        setState((s) => ({
          ...s,
          hoveredElement: el,
          hoveredRect: el.getBoundingClientRect(),
        }));
      }
    },
    [isActive]
  );

  const handleClick = useCallback(
    (e: MouseEvent) => {
      if (!isActive) return;

      const el = document.elementFromPoint(e.clientX, e.clientY) as HTMLElement | null;
      if (!el) return;

      if (el.closest('[data-studio-panel]') || el.closest('[data-picker-overlay]')) {
        return;
      }

      e.preventDefault();
      e.stopPropagation();

      const rect = el.getBoundingClientRect();
      setState((s) => ({
        ...s,
        selectedElement: el,
        selectedRect: rect,
      }));

      onSelect(el);
    },
    [isActive, onSelect]
  );

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (!isActive) return;
      if (e.key === 'Escape') {
        e.preventDefault();
        onDeactivate();
      }
    },
    [isActive, onDeactivate]
  );

  const updateRects = useCallback(() => {
    setState((s) => {
      const updates: Partial<PickerState> = {};
      if (s.hoveredElement && s.hoveredElement.isConnected) {
        updates.hoveredRect = s.hoveredElement.getBoundingClientRect();
      } else {
        updates.hoveredElement = null;
        updates.hoveredRect = null;
      }
      if (s.selectedElement && s.selectedElement.isConnected) {
        updates.selectedRect = s.selectedElement.getBoundingClientRect();
      } else {
        updates.selectedElement = null;
        updates.selectedRect = null;
      }
      return { ...s, ...updates };
    });
  }, []);

  useEffect(() => {
    if (!isActive) return;

    document.addEventListener('mousemove', handleMouseMove, true);
    document.addEventListener('click', handleClick, true);
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      document.removeEventListener('mousemove', handleMouseMove, true);
      document.removeEventListener('click', handleClick, true);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [isActive, handleMouseMove, handleClick, handleKeyDown]);

  useEffect(() => {
    if (!isActive && !state.selectedElement) return;

    window.addEventListener('scroll', updateRects, true);
    window.addEventListener('resize', updateRects);

    return () => {
      window.removeEventListener('scroll', updateRects, true);
      window.removeEventListener('resize', updateRects);
    };
  }, [isActive, !!state.selectedElement, updateRects]);

  const clearSelection = useCallback(() => {
    setState((s) => ({
      ...s,
      selectedElement: null,
      selectedRect: null,
    }));
  }, []);

  return {
    hoveredElement: state.hoveredElement,
    hoveredRect: state.hoveredRect,
    selectedElement: state.selectedElement,
    selectedRect: state.selectedRect,
    clearSelection,
  };
}
