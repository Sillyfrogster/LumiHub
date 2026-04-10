import { useQuery, useInfiniteQuery } from '@tanstack/react-query';
import { useWorldbookStore } from '../store/useWorldbookStore';
import { searchChubLorebooks } from '../api/chub';
import { listWorldbooks } from '../api/worldbooks';
import { fromChubLorebook, fromLumiHub } from '../types/worldbook';

const PAGE_SIZE = 48;

export function useWorldbooks() {
  const store = useWorldbookStore();
  const { source, search, sort, page, tags, excludeTags, showNsfw, showNsfl, authorSearch, infiniteScroll } = store;

  const tagsKey = tags.join(',');
  const excludeTagsKey = excludeTags.join(',');

  const fetchPage = async ({ pageParam = page }: { pageParam?: number }) => {
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
        total: res.pagination.total,
        totalPages: res.pagination.totalPages,
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
        creator: authorSearch || undefined,
      });
      return {
        worldbooks: res.nodes.map(fromChubLorebook),
        page: res.page,
        hasMore: res.hasMore,
        total: res.total,
        totalPages: Math.ceil(res.total / PAGE_SIZE),
      };
    }
  };

  const queryEnabled = !infiniteScroll;
  const query = useQuery({
    queryKey: ['worldbooks', source, search, sort, page, tagsKey, excludeTagsKey, showNsfw, showNsfl, authorSearch],
    queryFn: () => fetchPage({ pageParam: page }),
    enabled: queryEnabled,
    staleTime: 1000 * 60 * 5,
    placeholderData: (prev) => prev,
  });

  const infQueryEnabled = infiniteScroll;
  const infQuery = useInfiniteQuery({
    queryKey: ['worldbooks-inf', source, search, sort, tagsKey, excludeTagsKey, showNsfw, showNsfl, authorSearch],
    initialPageParam: 1,
    queryFn: fetchPage,
    getNextPageParam: (lastPage) => lastPage.hasMore ? lastPage.page + 1 : undefined,
    enabled: infQueryEnabled,
    staleTime: 1000 * 60 * 5,
  });

  if (infiniteScroll) {
    const lastPage = infQuery.data?.pages[infQuery.data.pages.length - 1];
    return {
      worldbooks: infQuery.data?.pages.flatMap(p => p.worldbooks) ?? [],
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
    worldbooks: data?.worldbooks ?? [],
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
