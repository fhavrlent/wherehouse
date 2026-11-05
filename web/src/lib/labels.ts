import { useSyncExternalStore } from "react";

/** One label waiting to be printed. A blank label has no name. */
export interface QueuedLabel {
  code: string;
  name: string;
  /** Where it is, e.g. "Home › Office". */
  detail: string;
}

/** Where a label's QR code leads. */
export const labelUrl = (code: string) => `${location.origin}/l/${code}`;

/** The code in a scanned QR code or a typed text, if there is one. */
export const codeFrom = (text: string): string | null => {
  const t = text.trim();
  const m = /\/l\/([A-Za-z0-9-]{1,20})\/?$/.exec(t);
  if (m?.[1]) return m[1].toUpperCase();
  return /^[A-Za-z0-9][A-Za-z0-9-]{0,19}$/.test(t) ? t.toUpperCase() : null;
};

// ---- the print list, kept in this browser ----

const KEY = "wherehouse-print-list";
const listeners = new Set<() => void>();
let cached: QueuedLabel[] | null = null;

const read = (): QueuedLabel[] => {
  if (cached) return cached;
  try {
    const raw = localStorage.getItem(KEY);
    cached = raw ? (JSON.parse(raw) as QueuedLabel[]) : [];
  } catch {
    cached = [];
  }
  return cached;
};

const write = (list: QueuedLabel[]) => {
  cached = list;
  try {
    localStorage.setItem(KEY, JSON.stringify(list));
  } catch {
    /* storage unavailable: the list lasts until the page is left */
  }
  for (const l of listeners) l();
};

export const printList = {
  get: read,
  /** Adds labels, replacing ones with the same code. */
  add: (labels: QueuedLabel[]) => {
    const codes = new Set(labels.map((l) => l.code));
    write([...read().filter((l) => !codes.has(l.code)), ...labels]);
  },
  remove: (code: string) => write(read().filter((l) => l.code !== code)),
  clear: () => write([]),
};

const subscribe = (fn: () => void) => {
  listeners.add(fn);
  const onStorage = (e: StorageEvent) => {
    if (e.key === KEY) {
      cached = null;
      fn();
    }
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(fn);
    window.removeEventListener("storage", onStorage);
  };
};

export const usePrintList = () => useSyncExternalStore(subscribe, read);

// ---- label sheets ----

export interface Sheet {
  id: string;
  /** Page size in mm. */
  page: [number, number];
  /** Label size in mm. */
  label: [number, number];
  columns: number;
  rows: number;
  /** Top and left page margins, and the gaps between labels, in mm. */
  margin: [number, number];
  gap: [number, number];
}

const a4x21: Sheet = {
  id: "a4-21",
  page: [210, 297],
  label: [63.5, 38.1],
  columns: 3,
  rows: 7,
  margin: [15.15, 7.25],
  gap: [0, 2.5],
};

export const SHEETS: Sheet[] = [
  a4x21,
  { id: "a4-24", page: [210, 297], label: [70, 37], columns: 3, rows: 8, margin: [0.5, 0], gap: [0, 0] },
  { id: "a4-65", page: [210, 297], label: [38.1, 21.2], columns: 5, rows: 13, margin: [10.7, 4.75], gap: [0, 2.5] },
  {
    id: "letter-30",
    page: [215.9, 279.4],
    label: [66.7, 25.4],
    columns: 3,
    rows: 10,
    margin: [12.7, 4.8],
    gap: [0, 3.2],
  },
  { id: "roll-62x29", page: [62, 29], label: [62, 29], columns: 1, rows: 1, margin: [0, 0], gap: [0, 0] },
  { id: "roll-90x29", page: [90, 29], label: [90, 29], columns: 1, rows: 1, margin: [0, 0], gap: [0, 0] },
];

const SHEET_KEY = "wherehouse-label-sheet";

export const savedSheet = (): Sheet => {
  try {
    const id = localStorage.getItem(SHEET_KEY);
    return SHEETS.find((s) => s.id === id) ?? a4x21;
  } catch {
    return a4x21;
  }
};

export const saveSheet = (id: string) => {
  try {
    localStorage.setItem(SHEET_KEY, id);
  } catch {
    /* ignore */
  }
};
