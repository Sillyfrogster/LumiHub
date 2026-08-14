"use client";

import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { Search, SlidersHorizontal, X } from "lucide-react";
import { useRouter } from "next/navigation";
import {
  type FormEvent,
  useEffect,
  useMemo,
  useState,
  useTransition,
} from "react";
import { Shell } from "@/components/layout/Shell";
import {
  assetKeys,
  type BrowseCursor,
  type BrowseFilters,
  type BrowseKind,
  type BrowsePage,
  fetchAssets,
  type NsfwVisibility,
  saveNsfwVisibility,
} from "@/lib/api/query";
import { useAuth } from "@/lib/auth";
import { BrowseCard } from "./BrowseCard";
import styles from "./BrowseResults.module.css";

const KINDS: Array<{ value?: BrowseKind; label: string }> = [
  { label: "All" },
  { value: "character", label: "Characters" },
  { value: "lorebook", label: "Lorebooks" },
  { value: "preset", label: "Presets" },
  { value: "theme", label: "Themes" },
];

const VISIBILITY: Array<{ value: NsfwVisibility; label: string }> = [
  { value: "hidden", label: "Hide" },
  { value: "blurred", label: "Blur" },
  { value: "shown", label: "Show" },
];

const STORED_VISIBILITY = "lumihub.nsfw-visibility";

function buildBrowseHref(filters: BrowseFilters) {
  const params = new URLSearchParams();
  if (filters.kind) params.set("kind", filters.kind);
  if (filters.platform) params.set("platform", filters.platform);
  if (filters.q) params.set("q", filters.q);
  for (const facet of filters.facet ?? []) params.append("facet", facet);
  const query = params.toString();
  return query ? `/browse?${query}` : "/browse";
}

function isVisibility(value: string | null): value is NsfwVisibility {
  return value === "hidden" || value === "blurred" || value === "shown";
}

export function BrowseResults({
  filters,
  initialPage,
}: {
  filters: BrowseFilters;
  initialPage: BrowsePage | null;
}) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { account } = useAuth();
  const [queryText, setQueryText] = useState(filters.q ?? "");
  const [visibilityOverride, setVisibilityOverride] =
    useState<NsfwVisibility>();
  const [preferenceError, setPreferenceError] = useState("");
  const [savingPreference, setSavingPreference] = useState(false);
  const [isNavigating, startNavigation] = useTransition();

  useEffect(() => setQueryText(filters.q ?? ""), [filters.q]);

  useEffect(() => {
    if (account !== null) {
      setVisibilityOverride(undefined);
      return;
    }
    const stored = window.localStorage.getItem(STORED_VISIBILITY);
    setVisibilityOverride(isVisibility(stored) ? stored : undefined);
  }, [account]);

  const query = useInfiniteQuery({
    queryKey: assetKeys.list(filters, visibilityOverride),
    queryFn: ({ pageParam }) =>
      fetchAssets({
        ...filters,
        limit: 24,
        nsfw: visibilityOverride,
        before: pageParam?.before,
        beforeId: pageParam?.beforeId,
      }),
    initialPageParam: null as BrowseCursor | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    initialData:
      initialPage && visibilityOverride === undefined
        ? { pages: [initialPage], pageParams: [null] }
        : undefined,
  });

  const pages = query.data?.pages;
  const overview = pages?.[0];
  const assets = useMemo(
    () => pages?.flatMap((page) => page.items) ?? [],
    [pages],
  );
  const activeVisibility =
    visibilityOverride ?? overview?.visibility ?? "blurred";
  const hasFilters = Boolean(
    filters.kind || filters.platform || filters.q || filters.facet?.length,
  );

  function navigate(next: BrowseFilters) {
    startNavigation(() =>
      router.push(buildBrowseHref(next), { scroll: false }),
    );
  }

  function submitSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    navigate({ ...filters, q: queryText.trim() || undefined });
  }

  function toggleFacet(key: string, value: string, selected: boolean) {
    const encoded = `${key}=${value}`;
    const facets = (filters.facet ?? []).filter((facet) => facet !== encoded);
    if (!selected) facets.push(encoded);
    navigate({ ...filters, facet: facets.length ? facets : undefined });
  }

  async function setPreference(next: NsfwVisibility) {
    if (account === undefined || savingPreference) return;
    setPreferenceError("");
    setSavingPreference(true);
    try {
      if (account) {
        await saveNsfwVisibility(next);
        await queryClient.invalidateQueries({ queryKey: assetKeys.all });
      } else {
        window.localStorage.setItem(STORED_VISIBILITY, next);
        setVisibilityOverride(next);
      }
    } catch {
      setPreferenceError("That preference could not be saved. Try again.");
    } finally {
      setSavingPreference(false);
    }
  }

  return (
    <Shell className={styles.shell}>
      <search>
        <form className={styles.searchForm} onSubmit={submitSearch}>
          <Search size={18} strokeWidth={1.5} aria-hidden="true" />
          <label htmlFor="browse-search" className="sr-only">
            Search the collection
          </label>
          <input
            id="browse-search"
            value={queryText}
            onChange={(event) => setQueryText(event.target.value)}
            placeholder="Search names, creators, and blurbs"
          />
          {queryText ? (
            <button
              type="button"
              className={styles.clearSearch}
              onClick={() => setQueryText("")}
              aria-label="Clear search"
            >
              <X size={16} aria-hidden="true" />
            </button>
          ) : null}
          <button type="submit" className={styles.searchButton}>
            Search
          </button>
        </form>
      </search>
      <p className={styles.searchHint}>
        Narrow further with <code>tag:fantasy</code> or{" "}
        <code>author:handle</code>.
      </p>

      <div className={styles.layout}>
        <aside className={styles.filters} aria-label="Browse filters">
          <div className={styles.filterHeading}>
            <span>
              <SlidersHorizontal size={15} aria-hidden="true" />
              Refine
            </span>
            {hasFilters ? (
              <button type="button" onClick={() => navigate({})}>
                Clear filters
              </button>
            ) : null}
          </div>

          <fieldset className={styles.filterGroup}>
            <legend>Kind</legend>
            <div className={styles.kindOptions}>
              {KINDS.map((kind) => {
                const selected = filters.kind === kind.value;
                return (
                  <button
                    key={kind.label}
                    type="button"
                    className={styles.filterButton}
                    aria-pressed={selected}
                    onClick={() =>
                      navigate({
                        ...filters,
                        kind: kind.value,
                        facet: undefined,
                      })
                    }
                  >
                    {kind.label}
                  </button>
                );
              })}
            </div>
          </fieldset>

          {overview?.platforms.length ? (
            <fieldset className={styles.filterGroup}>
              <legend>Platform</legend>
              <label className={styles.option}>
                <input
                  type="radio"
                  name="platform"
                  checked={!filters.platform}
                  onChange={() =>
                    navigate({
                      ...filters,
                      platform: undefined,
                      facet: undefined,
                    })
                  }
                />
                <span>Any platform</span>
              </label>
              {overview.platforms.map((platform) => (
                <label
                  key={platform.value}
                  className={styles.option}
                  data-disabled={platform.count === 0 || undefined}
                >
                  <input
                    type="radio"
                    name="platform"
                    value={platform.value}
                    checked={filters.platform === platform.value}
                    disabled={platform.count === 0}
                    onChange={() =>
                      navigate({
                        ...filters,
                        platform: platform.value,
                        facet: undefined,
                      })
                    }
                  />
                  <span>{platform.label}</span>
                  <small>{platform.count}</small>
                </label>
              ))}
            </fieldset>
          ) : null}

          {overview?.facets.map((facet) => (
            <fieldset key={facet.key} className={styles.filterGroup}>
              <legend>{facet.label}</legend>
              {facet.options.map((option) => (
                <label
                  key={option.value}
                  className={styles.option}
                  data-disabled={option.count === 0 || undefined}
                >
                  <input
                    type="checkbox"
                    checked={option.selected}
                    disabled={option.count === 0}
                    onChange={() =>
                      toggleFacet(facet.key, option.value, option.selected)
                    }
                  />
                  <span>{option.label}</span>
                  <small>{option.count}</small>
                </label>
              ))}
            </fieldset>
          ))}

          <fieldset className={styles.filterGroup}>
            <legend>Adult content</legend>
            <div className={styles.visibilityOptions}>
              {VISIBILITY.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  aria-pressed={activeVisibility === option.value}
                  disabled={account === undefined || savingPreference}
                  onClick={() => void setPreference(option.value)}
                >
                  {option.label}
                </button>
              ))}
            </div>
            <p className={styles.preferenceNote}>
              {account
                ? "Saved to your account."
                : "Kept on this browser for this visit."}
            </p>
            {preferenceError ? (
              <p className={styles.preferenceError} role="alert">
                {preferenceError}
              </p>
            ) : null}
          </fieldset>
        </aside>

        <section className={styles.results} aria-busy={isNavigating}>
          <header className={styles.resultsHeader}>
            <div>
              <h2>Newest additions</h2>
              {overview ? (
                <p aria-live="polite">
                  {overview.total === 1
                    ? "1 result"
                    : `${overview.total} results`}
                </p>
              ) : null}
            </div>
          </header>

          {query.isPending ? <LoadingGrid /> : null}
          {query.isError ? (
            <div className={styles.message} role="alert">
              <h3>The collection is out of reach.</h3>
              <p>Try the page again in a moment.</p>
              <button type="button" onClick={() => void query.refetch()}>
                Try again
              </button>
            </div>
          ) : null}
          {!query.isPending && !query.isError && overview?.total === 0 ? (
            <EmptyState
              state={overview.emptyState}
              suppressed={overview.suppressed}
              clear={() => navigate({})}
              show={() => void setPreference("shown")}
            />
          ) : null}
          {assets.length ? (
            <ul className={styles.grid}>
              {assets.map((asset) => (
                <BrowseCard key={asset.id} asset={asset} />
              ))}
            </ul>
          ) : null}

          {query.hasNextPage ? (
            <div className={styles.more}>
              <button
                type="button"
                onClick={() => void query.fetchNextPage()}
                disabled={query.isFetchingNextPage}
              >
                {query.isFetchingNextPage ? "Gathering more…" : "Load more"}
              </button>
            </div>
          ) : null}
        </section>
      </div>
    </Shell>
  );
}

function LoadingGrid() {
  const placeholders = ["one", "two", "three", "four", "five", "six"];
  return (
    <output className={styles.loadingGrid} aria-label="Loading the collection">
      {placeholders.map((placeholder) => (
        <div key={placeholder} className={styles.loadingCard} />
      ))}
    </output>
  );
}

function EmptyState({
  state,
  suppressed,
  clear,
  show,
}: {
  state: BrowsePage["emptyState"];
  suppressed: number;
  clear: () => void;
  show: () => void;
}) {
  if (state === "catalog") {
    return (
      <div className={styles.message}>
        <h3>The shelves are waiting.</h3>
        <p>The first creation will have plenty of room to shine.</p>
      </div>
    );
  }
  if (state === "suppressed") {
    return (
      <div className={styles.message}>
        <h3>Matching work is hidden.</h3>
        <p>
          {suppressed === 1
            ? "One result is outside your content preference."
            : `${suppressed} results are outside your content preference.`}
        </p>
        <button type="button" onClick={show}>
          Show adult content
        </button>
      </div>
    );
  }
  return (
    <div className={styles.message}>
      <h3>Nothing follows that path.</h3>
      <p>Try a broader search or clear the filters and begin again.</p>
      <button type="button" onClick={clear}>
        Clear filters
      </button>
    </div>
  );
}
