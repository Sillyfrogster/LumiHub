import { useQuery } from '@tanstack/react-query';
import { getChubCharacterDetail } from '../api/chub';

export function useChubCharacterDetail(fullPath: string | undefined) {
  return useQuery({
    queryKey: ['chub-character-detail', fullPath],
    queryFn: () => getChubCharacterDetail(fullPath!),
    enabled: !!fullPath,
    staleTime: 1000 * 60 * 10,
  });
}
