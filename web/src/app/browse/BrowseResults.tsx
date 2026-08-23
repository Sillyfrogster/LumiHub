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
import { buildBrowseHref } from "@/lib/browse-url";
import {
  readSessionVisibility,
  writeSessionVisibility,
} from "@/lib/nsfw-visibility";
import { BrowseCard } from "./BrowseCard";
import styles from "./BrowseResults.module.css";

const KINDS: Array<{ value?: BrowseKind; label: string }> = [
  { label: "All" },
  { value: "character", label: "Characters" },
  { value: "lorebook", label: "Lorebooks" },
  { value: "preset", label: "Presets" },
  { value: "theme", label: "Themes" },
  { value: "pack", label: "Packs" },
];

const VISIBILITY: Array<{ value: NsfwVisibility; label: string }> = [
  { value: "hidden", label: "Hide" },
  { value: "blurred", label: "Blur" },
  { value: "shown", label: "Show" },
];

export function BrowseResults({
  filters,
  initialPage,
  creator,
  basePath = "/browse",
  heading = "Catalog",
  headingVisuallyHidden = false,
}: {
  filters: BrowseFilters;
  initialPage: BrowsePage | null;
  creator?: string;
  basePath?: string;
  heading?: string;
  headingVisuallyHidden?: boolean;
}) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { account } = useAuth();
  const [queryText, setQueryText] = useState(filters.q ?? "");
  const [filtersOpen, setFiltersOpen] = useState(
    Boolean(filters.kind || filters.platform || filters.facet?.length),
  );
  const [visibilityOverride, setVisibilityOverride] =
    useState<NsfwVisibility>();
  const [preferenceError, setPreferenceError] = useState("");
  const [savingPreference, setSavingPreference] = useState(false);
  const [dismissedSuppression, setDismissedSuppression] = useState<string>();
  const [isNavigating, startNavigation] = useTransition();

  useEffect(() => setQueryText(filters.q ?? ""), [filters.q]);

  useEffect(() => {
    if (account !== null) {
      setVisibilityOverride(undefined);
      return;
    }
    setVisibilityOverride(readSessionVisibility());
  }, [account]);

  const query = useInfiniteQuery({
    queryKey: assetKeys.list(filters, visibilityOverride, creator),
    queryFn: ({ pageParam }) =>
      fetchAssets({
        ...filters,
        creator,
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
  const applicationOptions = overview?.platforms ?? [];
  const hasFilters = Boolean(
    filters.kind || filters.platform || filters.q || filters.facet?.length,
  );
  const suppressionKey =
    overview?.visibility === "hidden" && overview.suppressed > 0
      ? JSON.stringify([
          filters.kind ?? "",
          filters.platform ?? "",
          filters.q ?? "",
          filters.facet ?? [],
          overview.suppressed,
        ])
      : undefined;

  function navigate(next: BrowseFilters) {
    startNavigation(() =>
      router.push(buildBrowseHref(next, basePath), { scroll: false }),
    );
  }

  function submitSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    navigate({ ...filters, q: queryText || undefined });
  }

  function clearSearch() {
    setQueryText("");
    if (filters.q) navigate({ ...filters, q: undefined });
  }

  function toggleFacet(key: string, value: string, selected: boolean) {
    const encoded = `${key}=${value}`;
    const facets = (filters.facet ?? []).filter((facet) => facet !== encoded);
    if (!selected) facets.push(encoded);
    navigate({ ...filters, facet: facets.length ? facets : undefined });
  }

  function showContentSettings() {
    setFiltersOpen(true);
    requestAnimationFrame(() => {
      const settings = document.getElementById("adult-content-settings");
      settings?.scrollIntoView({ block: "center" });
      settings?.focus({ preventScroll: true });
    });
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
        writeSessionVisibility(next);
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
      <header className={styles.resultsHeader}>
        <div
          className={styles.resultsHeaderContent}
          data-compact={headingVisuallyHidden || undefined}
        >
          <div className={styles.resultSummary}>
            <h2
              id="browse-heading"
              className={headingVisuallyHidden ? "sr-only" : undefined}
            >
              {heading}
            </h2>
            {overview ? (
              <p aria-live="polite">
                {overview.total === 1
                  ? "1 result"
                  : `${overview.total} results`}
              </p>
            ) : null}
          </div>
          <div className={styles.searchRegion}>
            <search>
              <form className={styles.searchForm} onSubmit={submitSearch}>
                <Search size={18} strokeWidth={1.5} aria-hidden="true" />
                <label htmlFor="browse-search" className="sr-only">
                  {creator
                    ? `Search @${creator}'s creations`
                    : "Search the collection"}
                </label>
                <input
                  id="browse-search"
                  value={queryText}
                  onChange={(event) => setQueryText(event.target.value)}
                  placeholder={
                    creator
                      ? `Search @${creator}'s creations`
                      : "Search the collection"
                  }
                />
                {queryText ? (
                  <button
                    type="button"
                    className={styles.clearSearch}
                    onClick={clearSearch}
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
              Search names, creator handles, and blurbs. Use{" "}
              <code>tag:fantasy</code>
              {creator ? (
                "."
              ) : (
                <>
                  {" "}
                  or <code>author:handle</code>.
                </>
              )}
            </p>
          </div>
        </div>
      </header>

      <div className={styles.layout}>
        <button
          type="button"
          className={styles.filterToggle}
          aria-expanded={filtersOpen}
          aria-controls="browse-filters"
          onClick={() => setFiltersOpen((open) => !open)}
        >
          <span>
            <SlidersHorizontal size={16} aria-hidden="true" />
            {filtersOpen ? "Hide filters" : "Refine results"}
          </span>
          {hasFilters ? <small>Filters active</small> : null}
        </button>
        <aside
          id="browse-filters"
          className={styles.filters}
          aria-label="Browse filters"
          data-mobile-hidden={!filtersOpen || undefined}
        >
          <div className={styles.filterHeading}>
            <span>Filters</span>
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

          {applicationOptions.length ? (
            <fieldset className={styles.filterGroup}>
              <legend>Works with</legend>
              <label className={styles.option}>
                <input
                  type="radio"
                  name="platform"
                  checked={!filters.platform}
                  onChange={() => navigate({ ...filters, platform: undefined })}
                />
                <span>Any app</span>
              </label>
              {applicationOptions.map((platform) => (
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
                      navigate({ ...filters, platform: platform.value })
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
                    disabled={option.count === 0 && !option.selected}
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

          <fieldset
            id="adult-content-settings"
            className={styles.filterGroup}
            tabIndex={-1}
          >
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

          <button
            type="button"
            className={styles.finishFilters}
            onClick={() => setFiltersOpen(false)}
          >
            {overview
              ? `View ${overview.total} ${overview.total === 1 ? "result" : "results"}`
              : "View results"}
          </button>
        </aside>

        <section
          className={styles.results}
          aria-labelledby="browse-heading"
          aria-busy={isNavigating}
        >
          {suppressionKey && suppressionKey !== dismissedSuppression ? (
            <output
              className={styles.suppressionNotice}
              aria-label="Hidden results"
            >
              <span className={styles.suppressionNoticeContent}>
                <span className={styles.suppressionText}>
                  {overview.suppressed === 1
                    ? "1 matching result is hidden by your adult-content preference."
                    : `${overview.suppressed} matching results are hidden by your adult-content preference.`}
                </span>
                <button
                  type="button"
                  className={styles.settingLink}
                  onClick={showContentSettings}
                >
                  Review content setting
                </button>
              </span>
              <button
                type="button"
                className={styles.dismissSuppression}
                aria-label="Dismiss hidden-results notice"
                onClick={() => setDismissedSuppression(suppressionKey)}
              >
                <X size={16} aria-hidden="true" />
              </button>
            </output>
          ) : null}

          {query.isPending ? <LoadingGrid /> : null}
          {query.isError ? (
            <div className={styles.message} role="alert">
              <h3>The catalog could not load.</h3>
              <p>Check your connection, then try again.</p>
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
              {assets.map((asset, index) => (
                <BrowseCard
                  key={asset.id}
                  asset={asset}
                  visibility={activeVisibility}
                  eager={index === 0}
                />
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
                {query.isFetchingNextPage ? "Loading…" : "Load more"}
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
        <h3>No assets have been published.</h3>
        <p>The catalog will show new work here as it is published.</p>
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
      <h3>No results match these filters.</h3>
      <p>Broaden the search or clear the filters.</p>
      <button type="button" onClick={clear}>
        Clear filters
      </button>
    </div>
  );
}
