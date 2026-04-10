export interface BrowsePaginationState {
  page: number;
  total: number;
  totalPages: number;
  hasNextPage?: boolean;
  onPageChange?: (page: number) => void;
  loadingMore?: boolean;
}

export function resolveBrowsePagination(
  pagination: BrowsePaginationState,
  fallbackOnPageChange?: BrowsePaginationState['onPageChange'],
): BrowsePaginationState {
  return {
    ...pagination,
    onPageChange: pagination.onPageChange ?? fallbackOnPageChange,
    loadingMore: pagination.loadingMore ?? false,
  };
}
