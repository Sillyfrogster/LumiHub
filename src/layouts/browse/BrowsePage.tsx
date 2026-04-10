import React from 'react';
import { Search, X, SlidersHorizontal, Sparkles, ChevronLeft, ChevronRight, Loader2, Info } from 'lucide-react';
import clsx from 'clsx';
import styles from './BrowsePage.module.css';

interface PaginationProps {
  page: number;
  totalPages: number;
  total: number;
  hasNextPage?: boolean;
  onPageChange?: (page: number) => void;
  loadingMore?: boolean;
}

interface BrowsePageProps {
  title: string;
  sidebar: React.ReactNode;
  headerActions?: React.ReactNode;
  search: string;
  onSearchChange: (search: string) => void;
  onSearchSubmit: (e: React.FormEvent) => void;
  onClearSearch: () => void;
  searchPlaceholder?: string;
  children: React.ReactNode;
  loading: boolean;
  error: string | null;
  itemsCount: number;
  pagination: PaginationProps;
  emptyStateTitle?: string;
  emptyStateDesc?: string;
  /** Shown instead of the generic empty state when results exist but are hidden by filters. */
  filterBlockedMessage?: string;
  mobileFiltersOpen: boolean;
  onToggleMobileFilters: () => void;
  SkeletonGrid: React.ComponentType;
  infiniteScroll?: boolean;
  onToggleInfiniteScroll?: (enabled: boolean) => void;
}

const BrowsePage: React.FC<BrowsePageProps> = ({
  title,
  sidebar,
  headerActions,
  search,
  onSearchChange,
  onSearchSubmit,
  onClearSearch,
  searchPlaceholder = 'Search...',
  children,
  loading,
  error,
  itemsCount,
  pagination,
  emptyStateTitle = 'No results found',
  emptyStateDesc = 'Try adjusting your search or filters.',
  filterBlockedMessage,
  mobileFiltersOpen,
  onToggleMobileFilters,
  SkeletonGrid,
  infiniteScroll = false,
  onToggleInfiniteScroll,
}) => {
  const hasPrev = pagination.page > 1;
  const hasNext = pagination.hasNextPage ?? false;
  const canPage = hasPrev || hasNext;

  const observerRef = React.useRef<IntersectionObserver | null>(null);
  const sentinelRef = React.useCallback((node: HTMLDivElement | null) => {
    if (loading || pagination.loadingMore) return;
    if (observerRef.current) observerRef.current.disconnect();

    observerRef.current = new IntersectionObserver(entries => {
      if (entries[0].isIntersecting && pagination.hasNextPage && infiniteScroll) {
        pagination.onPageChange?.(pagination.page + 1);
      }
    });

    if (node) observerRef.current.observe(node);
  }, [loading, pagination.loadingMore, pagination.hasNextPage, infiniteScroll, pagination.onPageChange, pagination.page]);

  return (
    <div className={styles.page}>
      {/* Mobile Backdrop */}
      {mobileFiltersOpen && (
        <div className={styles.sidebarBackdrop} onClick={onToggleMobileFilters} />
      )}

      {/* Sidebar */}
      <aside className={clsx(styles.sidebar, mobileFiltersOpen && styles.sidebarOpen)}>
        <div className={styles.mobileSidebarHeader}>
          <h3>Filters</h3>
          <button onClick={onToggleMobileFilters} className={styles.mobileCloseBtn}>
            <X size={20} />
          </button>
        </div>
        <div className={styles.sidebarContent}>
          {sidebar}
        </div>
      </aside>

      {/* Content Area */}
      <div className={styles.content}>
        {/* Search + Title row */}
        <div className={styles.topBar}>
          <form onSubmit={onSearchSubmit} className={styles.searchForm}>
            <Search size={16} className={styles.searchIcon} />
            <input
              type="text"
              placeholder={searchPlaceholder}
              className={styles.searchInput}
              value={search}
              onChange={(e) => onSearchChange(e.target.value)}
            />
            {search && (
              <button type="button" className={styles.searchClear} onClick={onClearSearch}>
                <X size={14} />
              </button>
            )}
          </form>

          <div className={styles.topBarRow}>
            <h1 className={styles.pageTitle}>{title}</h1>

            <div className={styles.topBarActions}>
              {headerActions && (
                <div className={styles.headerActions}>
                  {headerActions}
                </div>
              )}

              {onToggleInfiniteScroll && (
                <div className={styles.toggleContainer}>
                  <span>Infinite Scroll</span>
                  <label className={styles.switch}>
                    <input 
                      type="checkbox" 
                      checked={infiniteScroll} 
                      onChange={(e) => onToggleInfiniteScroll(e.target.checked)} 
                    />
                    <span className={styles.slider}></span>
                  </label>
                </div>
              )}

              <button className={styles.mobileFilterBtn} onClick={onToggleMobileFilters}>
                <SlidersHorizontal size={16} />
                Filters
              </button>
            </div>
          </div>
        </div>

        {error && (
          <div className={styles.errorBanner}>{error}</div>
        )}

        {/* Result info */}
        {!loading && itemsCount > 0 && (
          <div className={styles.resultInfo}>
            {pagination.total > 0
              ? `Showing ${itemsCount} of ${pagination.total} results`
              : `Page ${pagination.page} · ${itemsCount} results`}
          </div>
        )}

        {/* Grid */}
        <div className={styles.grid}>
          {loading && itemsCount === 0 ? (
            <SkeletonGrid />
          ) : itemsCount > 0 ? (
            children
          ) : filterBlockedMessage ? (
            <div className={clsx(styles.emptyState, styles.filterBlocked)}>
              <Info size={28} />
              <h3>Results hidden by filters</h3>
              <p>{filterBlockedMessage}</p>
            </div>
          ) : (
            <div className={styles.emptyState}>
              <Sparkles size={28} />
              <h3>{emptyStateTitle}</h3>
              <p>{emptyStateDesc}</p>
            </div>
          )}
        </div>

        {/* Pagination controls */}
        {!loading && canPage && !infiniteScroll && (
          <div className={styles.pagination}>
            <button
              className={styles.pageBtn}
              disabled={!hasPrev || loading}
              onClick={() => pagination.onPageChange?.(pagination.page - 1)}
              aria-label="Previous page"
            >
              <ChevronLeft size={16} />
              Prev
            </button>

            <span className={styles.pageIndicator}>
              {pagination.loadingMore ? (
                <Loader2 size={14} className={styles.pageSpinner} />
              ) : (
                `Page ${pagination.page}`
              )}
            </span>

            <button
              className={styles.pageBtn}
              disabled={!hasNext || loading}
              onClick={() => pagination.onPageChange?.(pagination.page + 1)}
              aria-label="Next page"
            >
              Next
              <ChevronRight size={16} />
            </button>
          </div>
        )}

        {/* Infinite scroll */}
        {infiniteScroll && hasNext && (
          <div ref={sentinelRef} className={styles.sentinelWrap}>
            {pagination.loadingMore && <Loader2 size={24} className={styles.pageSpinner} />}
          </div>
        )}
      </div>
    </div>
  );
};

export default BrowsePage;
