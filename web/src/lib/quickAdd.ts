import { createContext, use } from "react";

/** Opens the quick add dialog, with a place to put things in. */
export type OpenQuickAdd = (placeId?: number | null) => void;

export const QuickAddContext = createContext<OpenQuickAdd>(() => undefined);

export const useQuickAdd = () => use(QuickAddContext);
