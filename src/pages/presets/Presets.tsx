import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { usePresetStore } from '../../store/usePresetStore';
import { usePresets } from '../../hooks/usePresets';
import { useAvailableTags } from '../../hooks/useAvailableTags';
import PresetCard from '../../components/presets/PresetCard';
import CreatePresetModal from '../../components/presets/CreatePresetModal';
import { Calendar, Download, Plus } from 'lucide-react';
import type { UnifiedPreset } from '../../types/preset';
import BrowsePage from '../../layouts/browse/BrowsePage';
import { resolveBrowsePagination } from '../../utils/browsePagination';
import {
  FilterSidebar,
  FilterSection,
  FilterSortList,
  FilterSortOption,
  FilterTagInput,
} from '../../layouts/browse/FilterSidebar';
import styles from './Presets.module.css';

const SORT_OPTIONS = [
  { key: 'created_at', label: 'Newest', icon: <Calendar size={14} /> },
  { key: 'downloads', label: 'Most Downloaded', icon: <Download size={14} /> },
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

const Presets = () => {
  const { user } = useAuth();
  const navigate = useNavigate();
  const {
    search, sort, tags,
    infiniteScroll,
    setSearch, setSort,
    addTag, removeTag,
    setInfiniteScroll,
  } = usePresetStore();

  const { presets, pagination, loading, error } = usePresets();

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [localSearch, setLocalSearch] = useState(search);
  const [mobileFilters, setMobileFilters] = useState(false);
  const [tagSearch, setTagSearch] = useState('');
  const { data: availableTags = [], isLoading: tagsLoading } = useAvailableTags('lumihub', 'presets', tagSearch);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (localSearch !== search) setSearch(localSearch);
    }, 400);
    return () => clearTimeout(timer);
  }, [localSearch, search, setSearch]);

  useEffect(() => { setLocalSearch(search); }, [search]);

  const handleSearchSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    setSearch(localSearch);
  }, [localSearch, setSearch]);

  return (
    <>
      <BrowsePage
        title="Presets"
        searchPlaceholder="Search presets..."
        headerActions={
          user ? (
            <button className={styles.createBtn} onClick={() => setIsCreateOpen(true)}>
              <Plus size={16} />
              Share
            </button>
          ) : undefined
        }
        emptyStateTitle="No presets yet"
        emptyStateDesc="Be the first to share a generation preset!"
        search={localSearch}
        onSearchChange={setLocalSearch}
        onSearchSubmit={handleSearchSubmit}
        onClearSearch={() => { setLocalSearch(''); setSearch(''); }}
        loading={loading}
        error={error}
        itemsCount={presets.length}
        infiniteScroll={infiniteScroll}
        onToggleInfiniteScroll={setInfiniteScroll}
        pagination={resolveBrowsePagination(pagination)}
        mobileFiltersOpen={mobileFilters}
        onToggleMobileFilters={() => setMobileFilters(!mobileFilters)}
        SkeletonGrid={SkeletonGrid}
        sidebar={
          <FilterSidebar>
            <FilterSection label="Sort By">
              <FilterSortList>
                {SORT_OPTIONS.map((opt) => (
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

            <FilterSection label="Tags">
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
          </FilterSidebar>
        }
      >
        {presets.map((preset: UnifiedPreset) => (
          <PresetCard
            key={preset.id}
            preset={preset}
            onClick={() => navigate(`/presets/${preset.id}`, { state: { preset } })}
          />
        ))}
      </BrowsePage>

      {isCreateOpen && (
        <CreatePresetModal onClose={() => setIsCreateOpen(false)} />
      )}
    </>
  );
};

export default Presets;
