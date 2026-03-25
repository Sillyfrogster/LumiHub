import { useInfiniteQuery } from '@tanstack/react-query';
import { useWorldbookStore } from '../store/useWorldbookStore';
import { searchChubLorebooks } from '../api/chub';
import { listWorldbooks } from '../api/worldbooks';
import { fromChubLorebook, fromLumiHub } from '../types/worldbook';
import { useMemo } from 'react';

const PAGE_SIZE = 48;

export function useWorldbooks() {
  const { source, search, sort, tags, excludeTags, showNsfw, showNsfl } = useWorldbookStore();

  const tagsKey = tags.join(',');
  const excludeTagsKey = excludeTags.join(',');

  const query = useInfiniteQuery({
    queryKey: ['worldbooks', source, search, sort, tagsKey, excludeTagsKey, showNsfw, showNsfl],
    queryFn: async ({ pageParam = 1 }) => {
      if (source === 'lumihub') {
        const res = await listWorldbooks({
          page: pageParam,
          limit: PAGE_SIZE,
          sort,
          order: 'desc',
          search: search || undefined,
          tags: tagsKey || undefined,
        });
        return {
          worldbooks: res.data.map(fromLumiHub),
          page: pageParam,
          hasMore: pageParam < res.pagination.totalPages,
        };
      } else {
        const res = await searchChubLorebooks({
          search: search || undefined,
          sort,
          limit: PAGE_SIZE,
          page: pageParam,
          nsfw: showNsfw,
          nsfl: showNsfl,
          tags: tagsKey || undefined,
          excludeTags: excludeTagsKey || undefined,
        });
        return {
          worldbooks: res.nodes.map(fromChubLorebook),
          page: res.page,
          hasMore: res.hasMore,
        };
      }
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage) => {
      if (lastPage.hasMore) return lastPage.page + 1;
      return undefined;
    },
    staleTime: 1000 * 60 * 5,
  });

  const allWorldbooks = useMemo(
    () => query.data?.pages.flatMap((p) => p.worldbooks) ?? [],
    [query.data],
  );

  return {
    worldbooks: allWorldbooks,
    loading: query.isLoading,
    loadingMore: query.isFetchingNextPage,
    hasNextPage: query.hasNextPage ?? false,
    fetchNextPage: query.fetchNextPage,
    error: query.error ? query.error.message : null,
  };
}
