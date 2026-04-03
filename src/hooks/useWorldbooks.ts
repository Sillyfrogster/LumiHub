import { useQuery } from '@tanstack/react-query';
import { useWorldbookStore } from '../store/useWorldbookStore';
import { searchChubLorebooks } from '../api/chub';
import { listWorldbooks } from '../api/worldbooks';
import { fromChubLorebook, fromLumiHub } from '../types/worldbook';

const PAGE_SIZE = 48;

export function useWorldbooks() {
  const { source, search, sort, page, tags, excludeTags, showNsfw, showNsfl, authorSearch } = useWorldbookStore();

  const tagsKey = tags.join(',');
  const excludeTagsKey = excludeTags.join(',');

  const query = useQuery({
    queryKey: ['worldbooks', source, search, sort, page, tagsKey, excludeTagsKey, showNsfw, showNsfl, authorSearch],
    queryFn: async () => {
      if (source === 'lumihub') {
        const res = await listWorldbooks({
          page,
          limit: PAGE_SIZE,
          sort,
          order: 'desc',
          search: search || undefined,
          tags: tagsKey || undefined,
        });
        return {
          worldbooks: res.data.map(fromLumiHub),
          page,
          hasMore: page < res.pagination.totalPages,
          totalPages: res.pagination.totalPages,
        };
      } else {
        const res = await searchChubLorebooks({
          search: search || undefined,
          sort,
          limit: PAGE_SIZE,
          page,
          nsfw: showNsfw,
          nsfl: showNsfl,
          tags: tagsKey || undefined,
          excludeTags: excludeTagsKey || undefined,
          creator: authorSearch || undefined,
        });
        return {
          worldbooks: res.nodes.map(fromChubLorebook),
          page: res.page,
          hasMore: res.hasMore,
          totalPages: 0,
        };
      }
    },
    staleTime: 1000 * 60 * 5,
    placeholderData: (prev) => prev,
  });

  const data = query.data;

  return {
    worldbooks: data?.worldbooks ?? [],
    pagination: {
      page: data?.page ?? page,
      totalPages: data?.totalPages ?? 0,
      hasNextPage: data?.hasMore ?? false,
    },
    loading: query.isLoading,
    loadingMore: query.isFetching && !query.isLoading,
    hasNextPage: data?.hasMore ?? false,
    error: query.error ? (query.error as Error).message : null,
  };
}
