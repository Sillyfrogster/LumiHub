import { useQuery } from '@tanstack/react-query';
import { getCharacterImages } from '../api/characters';

export function useCharacterImages(characterId: string | undefined) {
  return useQuery({
    queryKey: ['character-images', characterId],
    queryFn: () => getCharacterImages(characterId!),
    enabled: !!characterId,
    staleTime: 60_000,
  });
}
