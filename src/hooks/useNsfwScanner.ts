import { useEffect, useRef, useCallback } from 'react';
import * as nsfwjs from 'nsfwjs';

export interface NsfwPrediction {
  imgSrc: string;
  isNsfw: boolean;
  confidence: number;
  predictions?: Record<string, number>;
}

const modelCache = { current: null as nsfwjs.NSFWJS | null };
const scanCache = new Map<string, NsfwPrediction>();

export function useNsfwScanner() {
  const modelLoadingRef = useRef<Promise<nsfwjs.NSFWJS> | null>(null);

  const loadModel = useCallback(async () => {
    if (modelCache.current) return modelCache.current;
    if (modelLoadingRef.current) return modelLoadingRef.current;

    modelLoadingRef.current = nsfwjs.load();
    const model = await modelLoadingRef.current;
    modelCache.current = model;
    return model;
  }, []);

  const scanImage = useCallback(async (imgSrc: string): Promise<NsfwPrediction> => {
    if (scanCache.has(imgSrc)) {
      return scanCache.get(imgSrc)!;
    }

    try {
      const model = await loadModel();
      const img = new Image();
      img.crossOrigin = 'anonymous';
      img.src = imgSrc;

      await new Promise((resolve, reject) => {
        img.onload = resolve;
        img.onerror = reject;
        setTimeout(() => reject(new Error('Image load timeout')), 10000);
      });

      const predictions = await model.classify(img);
      const nsfwPred = predictions.find((p) => p.className === 'Hentai' || p.className === 'Porn' || p.className === 'Sexy');
      const confidence = nsfwPred?.probability || 0;
      const isNsfw = confidence > 0.6;

      const result: NsfwPrediction = {
        imgSrc,
        isNsfw,
        confidence,
        predictions: predictions.reduce((acc, p) => ({ ...acc, [p.className]: p.probability }), {}),
      };

      scanCache.set(imgSrc, result);
      return result;
    } catch (error) {
      console.warn('[NSFW Scanner] Failed to scan image:', error);
      return {
        imgSrc,
        isNsfw: false,
        confidence: 0,
      };
    }
  }, [loadModel]);

  return { scanImage, loadModel };
}
