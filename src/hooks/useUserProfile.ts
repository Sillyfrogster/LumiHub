import { useQuery } from '@tanstack/react-query';

export interface UserProfileData {
  id: string;
  discordId: string;
  username: string;
  displayName: string | null;
  avatar: string | null;
  banner: string | null;
  customCss: string | null;
  customHtml: string | null;
  role: string;
  createdAt: string;
}

export function useUserProfile(discordId: string) {
  return useQuery({
    queryKey: ['userProfile', discordId],
    queryFn: async (): Promise<UserProfileData | null> => {
      const res = await fetch(`/api/v1/user/profile/${encodeURIComponent(discordId)}`);
      if (!res.ok) {
        if (res.status === 404) return null;
        throw new Error('Failed to fetch user profile');
      }
      return res.json();
    },
    staleTime: 1000 * 60 * 5, // 5 minutes
    retry: 1,
  });
}
