import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { useCharacterStore } from '../../store/useCharacterStore';
import { useCharacters } from '../../hooks/useCharacters';
import { useAvailableTags } from '../../hooks/useAvailableTags';
import CharacterCard from '../../components/characters/CharacterCard';
import CreateCharacterModal from '../../components/characters/CreateCharacterModal';
import { Sparkles, Globe, Calendar, Download, CaseSensitive, Flame, Plus, User } from 'lucide-react';
import type { CharacterSource } from '../../types/character';
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
  FilterCheckbox,
  FilterNumberInput
} from '../../layouts/browse/FilterSidebar';
import styles from './Characters.module.css';

const SORT_OPTIONS_LUMIHUB = [
  { key: 'created_at', label: 'Newest', icon: <Calendar size={14} /> },
  { key: 'downloads', label: 'Most Downloaded', icon: <Download size={14} /> },
  { key: 'name', label: 'Alphabetical', icon: <CaseSensitive size={14} /> },
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

const Characters = () => {
  const { user } = useAuth();
  const {
    source, search, sort,
    tags, excludeTags,
    minTokens, showNsfw, showNsfl, requireImages, authorSearch,
    infiniteScroll,
    setSource, setSearch, setSort,
    addTag, removeTag, addExcludeTag, removeExcludeTag,
    setMinTokens, setShowNsfw, setShowNsfl, setRequireImages,
    setAuthorSearch, setInfiniteScroll,
    hydrateFromSettings,
  } = useCharacterStore();

  useEffect(() => {
    if (user?.settings) hydrateFromSettings(user.settings);
  }, [user?.settings, hydrateFromSettings]);

  const { characters, pagination, loading, error } = useCharacters();
  const navigate = useNavigate();

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [localSearch, setLocalSearch] = useState(search);
  const [localAuthorSearch, setLocalAuthorSearch] = useState(authorSearch);
  const [localMinTokens, setLocalMinTokens] = useState(minTokens);
  const [mobileFilters, setMobileFilters] = useState(false);
  const [tagSearch, setTagSearch] = useState('');
  const [excludeTagSearch, setExcludeTagSearch] = useState('');
  const { data: availableTags = [], isLoading: tagsLoading } = useAvailableTags(source, 'characters', tagSearch);
  const { data: availableExcludeTags = [], isLoading: excludeTagsLoading } = useAvailableTags(source, 'characters', excludeTagSearch);

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => {
      if (localSearch !== search) setSearch(localSearch);
    }, 400);
    return () => clearTimeout(timer);
  }, [localSearch, search, setSearch]);

  // Debounce author search (500ms)
  useEffect(() => {
    const timer = setTimeout(() => {
      if (localAuthorSearch !== authorSearch) setAuthorSearch(localAuthorSearch);
    }, 500);
    return () => clearTimeout(timer);
  }, [localAuthorSearch, authorSearch, setAuthorSearch]);

  // Debounce min tokens (2s)
  useEffect(() => {
    const timer = setTimeout(() => {
      if (localMinTokens !== minTokens) setMinTokens(localMinTokens);
    }, 2000);
    return () => clearTimeout(timer);
  }, [localMinTokens, minTokens, setMinTokens]);

  // Sync local state when store resets (e.g. source change)
  useEffect(() => { setLocalSearch(search); }, [search]);
  useEffect(() => { setLocalAuthorSearch(authorSearch); }, [authorSearch]);

  const handleSearchSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    setSearch(localSearch);
  }, [localSearch, setSearch]);

  const sortOptions = source === 'lumihub' ? SORT_OPTIONS_LUMIHUB : SORT_OPTIONS_CHUB;

  // Filter-blocked message: shown when Chub returns 0 results but filters are restrictive
  const isChub = source === 'chub';
  const hasActiveFilters = isChub && (minTokens > 0 || !showNsfw || !showNsfl);
  const filterBlockedMessage = (isChub && !loading && characters.length === 0 && (search || authorSearch) && hasActiveFilters)
    ? `Characters matching your search exist but may be hidden. Try lowering the minimum token count, or enabling NSFW/NSFL content in the filters.`
    : undefined;

  return (
    <>
      <BrowsePage
        title="Characters"
        searchPlaceholder="Search characters..."
        headerActions={source === 'lumihub' ? (
          <button
            className={styles.createBtn}
            onClick={() => setIsCreateOpen(true)}
          >
            <Plus size={16} />
            Create
          </button>
        ) : undefined}
        emptyStateTitle={source === 'lumihub' ? 'No characters yet' : 'No characters found'}
        emptyStateDesc={source === 'lumihub' ? 'Be the first to upload a character!' : 'Try adjusting your search or filters.'}
        search={localSearch}
        onSearchChange={setLocalSearch}
        onSearchSubmit={handleSearchSubmit}
        onClearSearch={() => {
          setLocalSearch('');
          setSearch('');
        }}
        loading={loading}
        error={error}
        itemsCount={characters.length}
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
                  onChange={(val) => setSource(val as CharacterSource)}
                />
                <FilterRadioOption
                  name="source"
                  value="chub"
                  label="Chub.ai"
                  icon={<Globe size={14} />}
                  checked={source === 'chub'}
                  onChange={(val) => setSource(val as CharacterSource)}
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
              <>
                <FilterSection label="Quality">
                  <FilterNumberInput
                    value={localMinTokens}
                    onChange={setLocalMinTokens}
                    min={0}
                    step={50}
                    suffix="min tokens"
                    placeholder="750"
                  />
                  <FilterCheckbox
                    label="Require avatar image"
                    checked={requireImages}
                    onChange={setRequireImages}
                  />
                </FilterSection>

                <FilterSection label="Content">
                  <FilterCheckbox
                    label="Show NSFW"
                    checked={showNsfw}
                    onChange={setShowNsfw}
                  />
                  <FilterCheckbox
                    label="Show NSFL"
                    checked={showNsfl}
                    onChange={setShowNsfl}
                  />
                </FilterSection>
              </>
            )}
          </FilterSidebar>
        }
      >
        {characters.map((card) => (
          <CharacterCard
            key={card.id}
            card={card}
            blurNsfw={!user?.settings?.nsfwUnblurred}
            onClick={() => navigate(`/characters/${encodeURIComponent(card.id)}`, { state: { card } })}
          />
        ))}
      </BrowsePage>

      {/* Create Modal */}
      {isCreateOpen && (
        <CreateCharacterModal onClose={() => setIsCreateOpen(false)} />
      )}
    </>
  );
};

export default Characters;
