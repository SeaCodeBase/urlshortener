import { useCallback } from 'react';
import { useIsMutating, useMutation, useQuery, useQueryClient, type MutationFilters } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { isReservedSlug, isValidSlug } from '@/components/pages/designPresets';
import type { BlockInput, BlockPatch, CreatePageInput, PageFilters, SlugCheck, UpdatePageInput } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';
import { useDebouncedValue } from './useDebouncedValue';

const SLUG_DEBOUNCE_MS = 300;

/** Pages of the active group. */
export function usePages(filters: PageFilters = {}) {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.pages(groupId, filters),
    queryFn: () => api.listPages(filters),
    enabled: groupId != null,
  });
}

export function usePage(id: number | null | undefined) {
  return useQuery({
    queryKey: queryKeys.page(id ?? 0),
    queryFn: () => api.getPage(id as number),
    enabled: id != null && id > 0,
  });
}

/** Mutation key of every editor save (page fields and blocks); publish is not one. */
export const PAGE_SAVE_MUTATION_KEY = ['page', 'save'] as const;

/** The page a save mutation writes: `{ id }` for page updates, `{ pageId }` for blocks. */
function savedPageId(variables: unknown): number | undefined {
  if (variables == null || typeof variables !== 'object') return undefined;
  const v = variables as { id?: unknown; pageId?: unknown };
  const id = 'pageId' in v ? v.pageId : v.id;
  return typeof id === 'number' ? id : undefined;
}

const pageSaveFilters = (pageId: number): MutationFilters => ({
  mutationKey: PAGE_SAVE_MUTATION_KEY,
  status: 'pending',
  predicate: (m) => savedPageId(m.state.variables) === pageId,
});

/** Number of in-flight saves (page fields or blocks) of the page. */
export function usePageSavesPending(pageId: number): number {
  return useIsMutating(pageSaveFilters(pageId));
}

/** Returns a function resolving once the page has no in-flight saves (at once if none). */
export function useWaitForPageSaves(pageId: number): () => Promise<void> {
  const qc = useQueryClient();
  return useCallback(
    () =>
      new Promise<void>((resolve) => {
        const idle = () => qc.isMutating(pageSaveFilters(pageId)) === 0;
        if (idle()) return resolve();
        const unsubscribe = qc.getMutationCache().subscribe(() => {
          if (!idle()) return;
          unsubscribe();
          resolve();
        });
      }),
    [qc, pageId],
  );
}

/**
 * Page and block mutations. Each invalidates the page list, the page (and its
 * stats) and links (link JSON carries `page_ids`; blocks can create links).
 * Content writes carry PAGE_SAVE_MUTATION_KEY so the editor can wait for them
 * before publishing.
 */
export function usePageMutations() {
  const qc = useQueryClient();
  const invalidate = (id?: number) => {
    qc.invalidateQueries({ queryKey: ['pages'] });
    qc.invalidateQueries({ queryKey: ['links'] });
    qc.invalidateQueries({ queryKey: ['link'] });
    if (id != null) qc.invalidateQueries({ queryKey: queryKeys.page(id) });
  };

  const create = useMutation({
    mutationFn: (input: CreatePageInput) => api.createPage(input),
    onSuccess: () => invalidate(),
  });

  const update = useMutation({
    mutationKey: PAGE_SAVE_MUTATION_KEY,
    mutationFn: ({ id, patch }: { id: number; patch: UpdatePageInput }) => api.updatePage(id, patch),
    onSuccess: (_page, { id }) => invalidate(id),
  });

  const setArchived = useMutation({
    mutationFn: ({ id, archived }: { id: number; archived: boolean }) => api.setPageArchived(id, archived),
    onSuccess: (_page, { id }) => invalidate(id),
  });

  const remove = useMutation({
    mutationFn: (id: number) => api.deletePage(id),
    onSuccess: (_data, id) => {
      qc.removeQueries({ queryKey: queryKeys.page(id) });
      invalidate();
    },
  });

  const publish = useMutation({
    mutationFn: (id: number) => api.publishPage(id),
    onSuccess: (page, id) => {
      qc.setQueryData(queryKeys.page(id), page);
      invalidate(id);
    },
  });

  const addBlock = useMutation({
    mutationKey: PAGE_SAVE_MUTATION_KEY,
    mutationFn: ({ pageId, input }: { pageId: number; input: BlockInput }) => api.addBlock(pageId, input),
    onSuccess: (_block, { pageId }) => invalidate(pageId),
  });

  const updateBlock = useMutation({
    mutationKey: PAGE_SAVE_MUTATION_KEY,
    mutationFn: ({ pageId, blockId, patch }: { pageId: number; blockId: number; patch: BlockPatch }) =>
      api.updateBlock(pageId, blockId, patch),
    onSuccess: (_block, { pageId }) => invalidate(pageId),
  });

  const deleteBlock = useMutation({
    mutationKey: PAGE_SAVE_MUTATION_KEY,
    mutationFn: ({ pageId, blockId }: { pageId: number; blockId: number }) => api.deleteBlock(pageId, blockId),
    onSuccess: (_data, { pageId }) => invalidate(pageId),
  });

  const reorder = useMutation({
    mutationKey: PAGE_SAVE_MUTATION_KEY,
    mutationFn: ({ pageId, ids }: { pageId: number; ids: number[] }) => api.reorderBlocks(pageId, ids),
    onSuccess: (_data, { pageId }) => invalidate(pageId),
  });

  return { create, update, setArchived, remove, publish, addBlock, updateBlock, deleteBlock, reorder };
}

export interface SlugCheckResult {
  /** The (debounced, lowercased) slug the result is for. */
  slug: string;
  /** null while pending, for an empty slug, or on error. */
  data: SlugCheck | null;
  /** True while the input is still settling or the request is in flight. */
  isChecking: boolean;
  error: Error | null;
}

/**
 * Availability of a slug, debounced 300 ms. Invalid or reserved slugs are
 * answered locally (`{available:false, reason:'invalid'|'reserved'}`) without
 * a request; an empty slug yields `data: null`.
 */
export function useSlugCheck(slug: string): SlugCheckResult {
  const normalized = slug.trim().toLowerCase();
  const debounced = useDebouncedValue(normalized, SLUG_DEBOUNCE_MS);
  const local: SlugCheck | null =
    debounced === ''
      ? null
      : !isValidSlug(debounced)
        ? { slug: debounced, available: false, reason: 'invalid' }
        : isReservedSlug(debounced)
          ? { slug: debounced, available: false, reason: 'reserved' }
          : null;
  const remote = debounced !== '' && local == null;

  const query = useQuery({
    queryKey: queryKeys.slugCheck(debounced),
    queryFn: () => api.checkSlug(debounced),
    enabled: remote,
    staleTime: 10_000,
  });

  const settling = normalized !== debounced;
  return {
    slug: debounced,
    data: local ?? (remote ? (query.data ?? null) : null),
    isChecking: settling || (remote && query.isFetching),
    error: remote ? query.error : null,
  };
}
