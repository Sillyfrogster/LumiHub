import { useQuery, useInfiniteQuery } from '@tanstack/react-query';
import { usePresetStore } from '../store/usePresetStore';
import { listPresets } from '../api/presets';
import { fromLumiHub } from '../types/preset';

const PAGE_SIZE = 48;

export function usePresets() {
  const store = usePresetStore();
  const { search, sort, page, tags, infiniteScroll } = store;

  const tagsKey = tags.join(',');

  const fetchPage = async ({ pageParam = page }: { pageParam?: number }) => {
    const res = await listPresets({
      page: pageParam,
      limit: PAGE_SIZE,
      sort,
      order: 'desc',
      search: search || undefined,
      tags: tagsKey || undefined,
    });
    return {
      presets: res.data.map(fromLumiHub),
      page: pageParam,
      hasMore: pageParam < res.pagination.totalPages,
      total: res.pagination.total,
      totalPages: res.pagination.totalPages,
    };
  };

  const queryEnabled = !infiniteScroll;
  const query = useQuery({
    queryKey: ['presets', search, sort, page, tagsKey],
    queryFn: () => fetchPage({ pageParam: page }),
    enabled: queryEnabled,
    staleTime: 1000 * 60 * 5,
    placeholderData: (prev) => prev,
  });

  const infQueryEnabled = infiniteScroll;
  const infQuery = useInfiniteQuery({
    queryKey: ['presets-inf', search, sort, tagsKey],
    initialPageParam: 1,
    queryFn: fetchPage,
    getNextPageParam: (lastPage) => lastPage.hasMore ? lastPage.page + 1 : undefined,
    enabled: infQueryEnabled,
    staleTime: 1000 * 60 * 5,
  });

  if (infiniteScroll) {
    const lastPage = infQuery.data?.pages[infQuery.data.pages.length - 1];
    return {
      presets: infQuery.data?.pages.flatMap(p => p.presets) ?? [],
      pagination: {
        page: lastPage?.page ?? 1,
        total: lastPage?.total ?? 0,
        totalPages: lastPage?.totalPages ?? 0,
        hasNextPage: infQuery.hasNextPage,
        onPageChange: () => {
          if (infQuery.hasNextPage && !infQuery.isFetchingNextPage) {
            infQuery.fetchNextPage();
          }
        },
        loadingMore: infQuery.isFetchingNextPage,
      },
      loading: infQuery.isLoading,
      loadingMore: infQuery.isFetchingNextPage,
      hasNextPage: !!infQuery.hasNextPage,
      error: infQuery.error ? (infQuery.error as Error).message : null,
    };
  }

  const data = query.data;

  return {
    presets: data?.presets ?? [],
    pagination: {
      page: data?.page ?? page,
      total: data?.total ?? 0,
      totalPages: data?.totalPages ?? 0,
      hasNextPage: data?.hasMore ?? false,
      onPageChange: store.setPage,
      loadingMore: query.isFetching && !query.isLoading,
    },
    loading: query.isLoading,
    loadingMore: query.isFetching && !query.isLoading,
    hasNextPage: data?.hasMore ?? false,
    error: query.error ? (query.error as Error).message : null,
  };
}
