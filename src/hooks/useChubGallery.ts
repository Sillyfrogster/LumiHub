import { useQuery } from '@tanstack/react-query';
import { getChubCharacterGallery } from '../api/chub';

/** Fetches gallery images for a Chub character. Only enabled when projectId is provided and hasGallery is true. */
export function useChubGallery(projectId: number | undefined, hasGallery: boolean | undefined) {
  return useQuery({
    queryKey: ['chub-gallery', projectId],
    queryFn: () => getChubCharacterGallery(projectId!),
    enabled: !!projectId && !!hasGallery,
    staleTime: 1000 * 60 * 10,
  });
}
