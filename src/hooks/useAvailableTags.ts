import { useQuery } from '@tanstack/react-query';
import { fetchChubTags } from '../api/chub';
import { listCharacterTags } from '../api/characters';
import { listWorldbookTags } from '../api/worldbooks';
import { listPresetTags } from '../api/presets';

export interface AvailableTag {
  name: string;
  count: number;
}

type Source = 'lumihub' | 'chub';
type ContentType = 'characters' | 'worldbooks' | 'presets';

/**
 * Fetches available tags for the given source and content type.
 * Supports debounced search via the `search` parameter.
 */
export function useAvailableTags(
  source: Source,
  contentType: ContentType,
  search: string,
) {
  return useQuery<AvailableTag[]>({
    queryKey: ['available-tags', source, contentType, search],
    queryFn: async () => {
      if (source === 'chub') {
        const namespace = contentType === 'worldbooks' ? 'lorebooks' : 'characters';
        return fetchChubTags(search || undefined, namespace);
      } else {
        if (contentType === 'worldbooks') {
          return listWorldbookTags(search || undefined);
        }
        if (contentType === 'presets') {
          return listPresetTags(search || undefined);
        }
        return listCharacterTags(search || undefined);
      }
    },
    staleTime: 1000 * 60 * 10,
    placeholderData: (prev) => prev,
  });
}
