import { keepPreviousData, QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { api, ApiError } from "./api";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (n, err) => !(err instanceof ApiError && (err.status === 401 || err.status === 404)) && n < 2,
    },
  },
});

export const keys = {
  me: ["me"] as const,
  settings: ["settings"] as const,
  home: ["home"] as const,
  places: ["places"] as const,
  thing: (id: number) => ["thing", id] as const,
  things: ["things"] as const,
  tags: ["tags"] as const,
  fields: ["fields"] as const,
  kinds: ["kinds"] as const,
  icons: ["icons"] as const,
  due: ["due"] as const,
  households: ["households"] as const,
  household: ["household"] as const,
};

export const useMe = () => useQuery({ queryKey: keys.me, queryFn: api.me, staleTime: Infinity });

export const useSettingsQuery = () => useQuery({ queryKey: keys.settings, queryFn: api.settings, staleTime: Infinity });

export const useSaveSettings = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.saveSettings,
    onSuccess: (s) => qc.setQueryData(keys.settings, s),
  });
};

export const useHome = () => useQuery({ queryKey: keys.home, queryFn: api.home });

/** Every place, ordered by path, e.g. for pickers. */
/** Every thing shown as a place, ordered by path, e.g. for pickers. */
export const usePlaces = () => useQuery({ queryKey: keys.places, queryFn: () => api.things({ places: true }) });

export const useThing = (id: number) => useQuery({ queryKey: keys.thing(id), queryFn: () => api.thing(id) });

export const useThings = (filter: Parameters<typeof api.things>[0]) =>
  useQuery({ queryKey: [...keys.things, filter], queryFn: () => api.things(filter) });

export const useKinds = () => useQuery({ queryKey: keys.kinds, queryFn: api.kinds, staleTime: 60_000 });

export const useHouseholds = () => useQuery({ queryKey: keys.households, queryFn: api.households, staleTime: 60_000 });

export const useHousehold = () => useQuery({ queryKey: keys.household, queryFn: api.household, staleTime: 60_000 });

/** Refreshes everything about things after a change. */
export const useInvalidate = () => {
  const qc = useQueryClient();
  return useCallback(() => {
    for (const key of [
      keys.home,
      keys.places,
      keys.things,
      keys.tags,
      keys.due,
      keys.kinds,
      ["thing"],
      ["tag"],
      ["search"],
      ["report"],
    ]) {
      void qc.invalidateQueries({ queryKey: key });
    }
  }, [qc]);
};

/** Searches as the query changes, keeping the last results until new ones arrive. */
export const useSearch = (q: string, archived: boolean) =>
  useQuery({
    queryKey: ["search", q, archived],
    queryFn: () => api.search(q, archived),
    enabled: q.trim() !== "",
    placeholderData: keepPreviousData,
  });

export const useTags = () => useQuery({ queryKey: keys.tags, queryFn: api.tags });

export const useTag = (id: number) => useQuery({ queryKey: ["tag", id], queryFn: () => api.tag(id) });

export const useFields = () => useQuery({ queryKey: keys.fields, queryFn: api.fields, staleTime: 60_000 });

/** Maintenance due within 30 days, and overdue. */
export const useDue = () => useQuery({ queryKey: keys.due, queryFn: () => api.dueMaintenance(30) });

export const useTemplates = () => useQuery({ queryKey: ["things", "templates"], queryFn: api.templates });
