import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { useWorldbookStore } from '../../store/useWorldbookStore';
import { useWorldbooks } from '../../hooks/useWorldbooks';
import { useAvailableTags } from '../../hooks/useAvailableTags';
import WorldbookCard from '../../components/worldbooks/WorldbookCard';
import CreateWorldbookModal from '../../components/worldbooks/CreateWorldbookModal';
import { Sparkles, Globe, Calendar, Flame, Download, Plus, User } from 'lucide-react';
import type { WorldBookSource, UnifiedWorldBook } from '../../types/worldbook';
import BrowsePage from '../../layouts/browse/BrowsePage';
import { resolveBrowsePagination } from '../../utils/browsePagination';
import {
  FilterSidebar,
  FilterSection,
  FilterRadioGroup,
  FilterRadioOption,
  FilterSortList,
  FilterSortOption,
  FilterTagInput,
  FilterCheckbox
} from '../../layouts/browse/FilterSidebar';
import styles from './Worldbooks.module.css';

const SORT_OPTIONS_LUMIHUB = [
  { key: 'created_at', label: 'Newest', icon: <Calendar size={14} /> },
  { key: 'download_count', label: 'Most Downloaded', icon: <Download size={14} /> },
];

const SORT_OPTIONS_CHUB = [
  { key: 'trending', label: 'Trending', icon: <Flame size={14} /> },
  { key: 'created_at', label: 'Newest', icon: <Calendar size={14} /> },
  { key: 'download_count', label: 'Most Downloaded', icon: <Download size={14} /> },
];

function SkeletonGrid() {
  return (
    <>
      {Array.from({ length: 12 }).map((_, i) => (
        <div key={i} className={styles.skeleton}>
          <div className={styles.skeletonShimmer} />
        </div>
      ))}
    </>
  );
}

const Worldbooks = () => {
  const { user } = useAuth();
  const navigate = useNavigate();
  const {
    source, search, sort,
    tags, excludeTags,
    showNsfw, showNsfl, authorSearch, infiniteScroll,
    setSource, setSearch, setSort,
    addTag, removeTag, addExcludeTag, removeExcludeTag,
    setShowNsfw, setShowNsfl,
    setAuthorSearch, setInfiniteScroll,
    hydrateFromSettings,
  } = useWorldbookStore();

  useEffect(() => {
    if (user?.settings) hydrateFromSettings(user.settings);
  }, [user?.settings, hydrateFromSettings]);

  const { worldbooks, pagination, loading, error } = useWorldbooks();

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [localSearch, setLocalSearch] = useState(search);
  const [localAuthorSearch, setLocalAuthorSearch] = useState(authorSearch);
  const [mobileFilters, setMobileFilters] = useState(false);
  const [tagSearch, setTagSearch] = useState('');
  const [excludeTagSearch, setExcludeTagSearch] = useState('');
  const { data: availableTags = [], isLoading: tagsLoading } = useAvailableTags(source, 'worldbooks', tagSearch);
  const { data: availableExcludeTags = [], isLoading: excludeTagsLoading } = useAvailableTags(source, 'worldbooks', excludeTagSearch);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (localSearch !== search) setSearch(localSearch);
    }, 400);
    return () => clearTimeout(timer);
  }, [localSearch, search, setSearch]);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (localAuthorSearch !== authorSearch) setAuthorSearch(localAuthorSearch);
    }, 500);
    return () => clearTimeout(timer);
  }, [localAuthorSearch, authorSearch, setAuthorSearch]);

  useEffect(() => { setLocalSearch(search); }, [search]);
  useEffect(() => { setLocalAuthorSearch(authorSearch); }, [authorSearch]);

  const handleSearchSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    setSearch(localSearch);
  }, [localSearch, setSearch]);

  const sortOptions = source === 'lumihub' ? SORT_OPTIONS_LUMIHUB : SORT_OPTIONS_CHUB;

  const isChub = source === 'chub';
  const hasActiveFilters = isChub && (!showNsfw || !showNsfl);
  const filterBlockedMessage = (isChub && !loading && worldbooks.length === 0 && (search || localAuthorSearch) && hasActiveFilters)
    ? `Worldbooks matching your search may be hidden by your content filters. Try enabling NSFW or NSFL to see more results.`
    : undefined;

  return (
    <>
    <BrowsePage
      title="Worldbooks"
      searchPlaceholder="Search worldbooks..."
      headerActions={source === 'lumihub' ? (
        <button className={styles.createBtn} onClick={() => setIsCreateOpen(true)}>
          <Plus size={16} />
          Upload
        </button>
      ) : undefined}
      emptyStateTitle={source === 'lumihub' ? 'No worldbooks yet' : 'No worldbooks found'}
      emptyStateDesc={source === 'lumihub' ? 'Be the first to upload a worldbook!' : 'Try adjusting your search or filters.'}
      search={localSearch}
      onSearchChange={setLocalSearch}
      onSearchSubmit={handleSearchSubmit}
      onClearSearch={() => { setLocalSearch(''); setSearch(''); }}
      loading={loading}
      error={error}
      itemsCount={worldbooks.length}
      filterBlockedMessage={filterBlockedMessage}
      infiniteScroll={infiniteScroll}
      onToggleInfiniteScroll={setInfiniteScroll}
      pagination={resolveBrowsePagination(pagination)}
      mobileFiltersOpen={mobileFilters}
      onToggleMobileFilters={() => setMobileFilters(!mobileFilters)}
      SkeletonGrid={SkeletonGrid}
      sidebar={
        <FilterSidebar>
          <FilterSection label="Source">
            <FilterRadioGroup>
              <FilterRadioOption
                name="source"
                value="lumihub"
                label="LumiHub"
                icon={<Sparkles size={14} />}
                checked={source === 'lumihub'}
                onChange={(val) => setSource(val as WorldBookSource)}
              />
              <FilterRadioOption
                name="source"
                value="chub"
                label="Chub.ai"
                icon={<Globe size={14} />}
                checked={source === 'chub'}
                onChange={(val) => setSource(val as WorldBookSource)}
              />
            </FilterRadioGroup>
          </FilterSection>

          <FilterSection label="Sort By">
            <FilterSortList>
              {sortOptions.map((opt) => (
                <FilterSortOption
                  key={opt.key}
                  label={opt.label}
                  icon={opt.icon}
                  active={sort === opt.key}
                  onClick={() => setSort(opt.key)}
                />
              ))}
            </FilterSortList>
          </FilterSection>

          {source === 'chub' && (
            <FilterSection label="Author">
              <div className={styles.authorInputWrap}>
                <User size={13} className={styles.authorIcon} />
                <input
                  type="text"
                  className={styles.authorInput}
                  placeholder="Filter by username…"
                  value={localAuthorSearch}
                  onChange={(e) => setLocalAuthorSearch(e.target.value)}
                />
                {localAuthorSearch && (
                  <button
                    className={styles.authorClear}
                    onClick={() => { setLocalAuthorSearch(''); setAuthorSearch(''); }}
                    type="button"
                    aria-label="Clear author filter"
                  >
                    ×
                  </button>
                )}
              </div>
            </FilterSection>
          )}

          <FilterSection label="Include Tags">
            <FilterTagInput
              tags={tags}
              onAdd={addTag}
              onRemove={removeTag}
              availableTags={availableTags}
              loading={tagsLoading}
              onSearchChange={setTagSearch}
              placeholder="Search tags…"
              variant="include"
            />
          </FilterSection>

          {source === 'chub' && (
            <FilterSection label="Exclude Tags">
              <FilterTagInput
                tags={excludeTags}
                onAdd={addExcludeTag}
                onRemove={removeExcludeTag}
                availableTags={availableExcludeTags}
                loading={excludeTagsLoading}
                onSearchChange={setExcludeTagSearch}
                placeholder="Search tags…"
                variant="exclude"
              />
            </FilterSection>
          )}

          {source === 'chub' && (
            <FilterSection label="Content">
              <FilterCheckbox label="Show NSFW" checked={showNsfw} onChange={setShowNsfw} />
              <FilterCheckbox label="Show NSFL" checked={showNsfl} onChange={setShowNsfl} />
            </FilterSection>
          )}
        </FilterSidebar>
      }
    >
      {worldbooks.map((wb: UnifiedWorldBook) => (
        <WorldbookCard
          key={wb.id}
          worldbook={wb}
          blurNsfw={!user?.settings?.nsfwUnblurred}
          onClick={() => navigate(`/worldbooks/${encodeURIComponent(wb.id)}`, { state: { worldbook: wb } })}
        />
      ))}
    </BrowsePage>

    {isCreateOpen && (
      <CreateWorldbookModal onClose={() => setIsCreateOpen(false)} />
    )}
    </>
  );
};

export default Worldbooks;
