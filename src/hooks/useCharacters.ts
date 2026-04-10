import { useQuery, useInfiniteQuery } from '@tanstack/react-query';
import { useCharacterStore } from '../store/useCharacterStore';
import { listCharacters } from '../api/characters';
import { searchChubCharacters, transformChubCharacter } from '../api/chub';
import { fromLumiHub, fromChub } from '../types/character';

const PAGE_SIZE = 48;

export function useCharacters(params: { ownerId?: string, ignoreStore?: boolean, enabled?: boolean } = {}) {
  const store = useCharacterStore();
  const source = params.ignoreStore ? 'lumihub' : store.source;
  const search = params.ignoreStore ? '' : store.search;
  const sort = params.ignoreStore ? 'created_at' : store.sort;
  const page = params.ignoreStore ? 1 : store.page;
  const infiniteScroll = params.ignoreStore ? false : store.infiniteScroll;
  const tags = params.ignoreStore ? [] : store.tags;
  const excludeTags = params.ignoreStore ? [] : store.excludeTags;
  const minTokens = params.ignoreStore ? 0 : store.minTokens;
  const showNsfw = params.ignoreStore ? true : store.showNsfw;
  const showNsfl = params.ignoreStore ? true : store.showNsfl;
  const requireImages = params.ignoreStore ? false : store.requireImages;
  const authorSearch = params.ignoreStore ? '' : store.authorSearch;

  const tagsKey = tags.join(',');
  const excludeTagsKey = excludeTags.join(',');

  const fetchPage = async ({ pageParam = page }: { pageParam?: number }) => {
    if (source === 'lumihub') {
      const res = await listCharacters({
        page: pageParam,
        limit: PAGE_SIZE,
        sort,
        order: 'desc',
        search: search || undefined,
        tags: tagsKey || undefined,
        ownerId: params.ownerId,
      });
      return {
        characters: res.data.map(fromLumiHub),
        page: pageParam,
        hasMore: pageParam < res.pagination.totalPages,
        total: res.pagination.total,
        totalPages: res.pagination.totalPages,
      };
    } else {
      const res = await searchChubCharacters({
        search: search || undefined,
        sort: sort as any,
        limit: PAGE_SIZE,
        page: pageParam,
        nsfw: showNsfw,
        nsfl: showNsfl,
        minTokens,
        requireImages,
        tags: tagsKey || undefined,
        excludeTags: excludeTagsKey || undefined,
        creator: authorSearch || undefined,
      });
      // chub is one some bullshit.
      const hasMore = res.nodes.length > 0 && pageParam < 100;
      return {
        characters: res.nodes.map(transformChubCharacter).map(fromChub),
        page: res.page,
        hasMore,
        total: 0,
        totalPages: 0,
      };
    }
  };

  const queryEnabled = params.enabled !== false && !infiniteScroll;
  const query = useQuery({
    queryKey: ['characters', source, search, sort, page, tagsKey, excludeTagsKey, minTokens, showNsfw, showNsfl, requireImages, authorSearch, params.ownerId],
    queryFn: () => fetchPage({ pageParam: page }),
    enabled: queryEnabled,
    staleTime: 1000 * 60 * 5,
    placeholderData: (prev) => prev,
  });

  const infQueryEnabled = params.enabled !== false && infiniteScroll;
  const infQuery = useInfiniteQuery({
    queryKey: ['characters-inf', source, search, sort, tagsKey, excludeTagsKey, minTokens, showNsfw, showNsfl, requireImages, authorSearch, params.ownerId],
    initialPageParam: 1,
    queryFn: fetchPage,
    getNextPageParam: (lastPage) => lastPage.hasMore ? lastPage.page + 1 : undefined,
    enabled: infQueryEnabled,
    staleTime: 1000 * 60 * 5,
  });

  if (infiniteScroll) {
    const lastPage = infQuery.data?.pages[infQuery.data.pages.length - 1];
    return {
      characters: infQuery.data?.pages.flatMap(p => p.characters) ?? [],
      pagination: {
        page: lastPage?.page ?? 1,
        limit: PAGE_SIZE,
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
    characters: data?.characters ?? [],
    pagination: {
      page: data?.page ?? page,
      limit: PAGE_SIZE,
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
