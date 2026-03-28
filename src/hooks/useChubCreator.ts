import { useQuery } from '@tanstack/react-query';
import { getChubCreatorProfile } from '../api/chub';

/** Fetches a Chub creator's profile. Only enabled when username is provided and source is Chub. */
export function useChubCreator(username: string | undefined) {
  return useQuery({
    queryKey: ['chub-creator', username],
    queryFn: () => getChubCreatorProfile(username!),
    enabled: !!username,
    staleTime: 1000 * 60 * 10,
  });
}
