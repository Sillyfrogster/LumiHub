import { useQuery } from '@tanstack/react-query';
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
  const tags = store.tags;
  const excludeTags = store.excludeTags;
  const minTokens = store.minTokens;
  const showNsfw = store.showNsfw;
  const showNsfl = store.showNsfl;
  const requireImages = store.requireImages;
  const authorSearch = params.ignoreStore ? '' : store.authorSearch;

  const tagsKey = tags.join(',');
  const excludeTagsKey = excludeTags.join(',');

  const query = useQuery({
    queryKey: ['characters', source, search, sort, page, tagsKey, excludeTagsKey, minTokens, showNsfw, showNsfl, requireImages, authorSearch, params.ownerId],
    queryFn: async () => {
      if (source === 'lumihub') {
        const res = await listCharacters({
          page,
          limit: PAGE_SIZE,
          sort,
          order: 'desc',
          search: search || undefined,
          tags: tagsKey || undefined,
          ownerId: params.ownerId,
        });
        return {
          characters: res.data.map(fromLumiHub),
          page,
          hasMore: page < res.pagination.totalPages,
          total: res.pagination.total,
          totalPages: res.pagination.totalPages,
        };
      } else {
        const res = await searchChubCharacters({
          search: search || undefined,
          sort: sort as any,
          limit: PAGE_SIZE,
          page,
          nsfw: showNsfw,
          nsfl: showNsfl,
          minTokens,
          requireImages,
          tags: tagsKey || undefined,
          excludeTags: excludeTagsKey || undefined,
          creator: authorSearch || undefined,
        });
        return {
          characters: res.nodes.map(transformChubCharacter).map(fromChub),
          page: res.page,
          hasMore: res.hasMore,
          total: 0,
          totalPages: 0,
        };
      }
    },
    enabled: params.enabled !== false,
    staleTime: 1000 * 60 * 5,
    placeholderData: (prev) => prev,
  });

  const data = query.data;

  return {
    characters: data?.characters ?? [],
    pagination: {
      page: data?.page ?? page,
      limit: PAGE_SIZE,
      total: data?.total ?? 0,
      totalPages: data?.totalPages ?? 0,
      hasNextPage: data?.hasMore ?? false,
    },
    loading: query.isLoading,
    loadingMore: query.isFetching && !query.isLoading,
    hasNextPage: data?.hasMore ?? false,
    error: query.error ? (query.error as Error).message : null,
  };
}
