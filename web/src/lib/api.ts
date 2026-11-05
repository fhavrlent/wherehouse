import i18n from "@/i18n";

export interface KindRef {
  id: number;
  /** A built-in kind's key ("building", "room", "furniture", "box", "thing"); "" for added ones. */
  key: string;
  /** "" for a built-in kind that wasn't renamed; show its translated name. */
  name: string;
  /** A built-in icon's name, or "custom:<id>". */
  icon: string;
  isPlace: boolean;
}

export interface Kind extends KindRef {
  position: number;
  /** How many things are of this kind. */
  count: number;
}

export type KindInput = Pick<Kind, "name" | "icon" | "isPlace" | "position">;

export interface CustomIcon {
  id: number;
  mimeType: string;
}

/** Names a thing in another's path. */
export interface Ref {
  id: number;
  name: string;
  icon: string;
}

export interface Photo {
  id: number;
  thingId: number;
  mimeType: string;
  size: number;
  createdAt: string;
}

export const TAG_COLORS = [
  "slate",
  "red",
  "orange",
  "amber",
  "lime",
  "green",
  "teal",
  "sky",
  "blue",
  "violet",
  "pink",
] as const;
export type TagColor = (typeof TAG_COLORS)[number];

export interface Tag {
  id: number;
  name: string;
  color: TagColor;
  /** How many current things have it, in tag lists. */
  count?: number;
}

export type FieldType = "text" | "number" | "date" | "url" | "bool";
export const FIELD_TYPES: readonly FieldType[] = ["text", "number", "date", "url", "bool"];

export interface Field {
  id: number;
  name: string;
  type: FieldType;
  position: number;
}

export interface FieldValue {
  fieldId: number;
  name: string;
  type: FieldType;
  value: string;
}

export type AttachmentKind = "manual" | "receipt" | "warranty" | "other";
export const ATTACHMENT_KINDS: readonly AttachmentKind[] = ["manual", "receipt", "warranty", "other"];

export interface Attachment {
  id: number;
  thingId: number;
  kind: AttachmentKind;
  storage: "local" | "paperless";
  filename: string;
  mimeType: string;
  size: number;
  paperlessDocId: number | null;
  createdAt: string;
}

export interface Maintenance {
  id: number;
  thingId: number;
  title: string;
  doneOn: string | null;
  nextDue: string | null;
  notes: string;
  cost: number | null;
  createdAt: string;
  updatedAt: string;
  /** In lists of what is due. */
  thingName?: string;
  path?: Ref[];
}

export type MaintenanceInput = Pick<Maintenance, "title" | "doneOn" | "nextDue" | "notes" | "cost">;

export interface Link {
  id: number;
  thingId: number;
  url: string;
  title: string;
  /** The app the link is from, such as "warranteed"; "" for links people added. */
  source: string;
  externalId?: string;
  createdAt: string;
}

/** What a thing cost, its warranty and insurance, and whether it was sold. Prices are of one piece. */
export interface Purchase {
  purchaseDate: string | null;
  purchasePrice: number | null;
  purchaseFrom: string;
  /** "" is the household's currency. */
  currency: string;
  warrantyUntil: string | null;
  lifetimeWarranty: boolean;
  warrantyDetails: string;
  insured: boolean;
  soldDate: string | null;
  soldPrice: number | null;
  soldTo: string;
  soldNotes: string;
}

/** Anything: a building, a room, a box, or what is in it. */
export interface Thing extends Purchase {
  id: number;
  parentId: number | null;
  kind: KindRef;
  name: string;
  description: string;
  quantity: number;
  manufacturer: string;
  model: string;
  serial: string;
  notes: string;
  template: boolean;
  /** The app whose purchase and warranty details these are, "" for this one. */
  purchaseOwner: string;
  archivedAt: string | null;
  /** The things it is in, outermost first. */
  path: Ref[];
  /** How many current things are directly inside. */
  contains: number;
  cover: number | null;
  /** The code of its printed label, or "". */
  label: string;
  tags: Tag[];
  /** Filled in for one thing, not in lists. */
  photos?: Photo[];
  fields?: FieldValue[];
  attachments?: Attachment[];
  maintenance?: Maintenance[];
  links?: Link[];
  createdAt: string;
  updatedAt: string;
}

export interface ThingDetail extends Thing {
  /** What is directly inside, places first. */
  contents: Thing[];
}

export type ThingInput = Pick<
  Thing,
  "parentId" | "name" | "description" | "quantity" | "manufacturer" | "model" | "serial" | "notes"
> &
  Partial<Purchase> & {
    /** 0 is the built-in "thing" kind. */
    kindId: number;
    /** Tag names; omitted keeps the thing's tags. */
    tags?: string[];
    /** Values by field ID; fields left out keep their values, "" clears one. */
    fields?: Record<number, string>;
  };

/** Whether a thing shows as a place: its kind is one, or it holds something. */
export const isPlace = (t: Pick<Thing, "kind" | "contains">) => t.kind.isPlace || t.contains > 0;

export interface Home {
  /** The top-level places. */
  places: Thing[];
  recent: Thing[];
  /** How many things aren't in anything yet. */
  loose: number;
}

/** Amounts by currency. */
export type Money = Record<string, number>;

export interface Group {
  id: number;
  name: string;
  icon?: string;
  color?: string;
  key?: string;
  value: Money;
  count: number;
}

export interface Report {
  currency: string;
  value: Money;
  insured: Money;
  things: number;
  priced: number;
  byPlace: Group[];
  byTag: Group[];
  byKind: Group[];
  warrantyEnding: Thing[];
  sold: Money;
}

export interface Household {
  id: number;
  name: string;
  currency: string;
  role: "owner" | "member";
  members: number;
}

export interface Member {
  userId: number;
  name: string;
  email: string;
  role: "owner" | "member";
  joinedAt: string;
}

/** What a label is on, or the other household it is of. */
export interface LabelInfo {
  code: string;
  thing?: Thing;
  household?: Household;
}

export interface ImportProgress {
  running: boolean;
  total: number;
  done: number;
  things: number;
  photos: number;
  documents: number;
  links: number;
  maintenance: number;
  skipped: number;
  errors: string[];
  error?: string;
  finished?: string;
}

export interface ApiToken {
  id: number;
  household: number;
  name: string;
  createdAt: string;
  lastUsedAt: string | null;
}

export interface PaperlessDoc {
  id: number;
  title: string;
  created: string;
  originalFileName: string;
  mimeType: string;
}

export interface Settings {
  theme: "system" | "light" | "dark";
  /** Language code, or "" to follow the browser. */
  language: string;
  /** Shoutrrr URLs, one per line. */
  notifyUrls: string;
  /** Days before maintenance is due that reminders go out, e.g. "7,0". */
  notifyDays: string;
  isAdmin: boolean;
  /** Languages the server has translations for. */
  languages: string[];
  timezone: string;
  /** Hosts non-admins may use with self-hosted notification services. */
  notifyAllowedHosts: string[];
  /** Where the browser opens the server's Paperless, "" when there is none. */
  paperlessUrl: string;
  hasPaperlessToken: boolean;
  /** An admin can use the server's own Paperless token. */
  paperlessServerToken: boolean;
  paperlessEnabled: boolean;
  /** Documents linked from Paperless get a link back here. */
  paperlessBacklinks: boolean;
  /** The Paperless custom field those links go in. */
  paperlessField: string;
}

export type SettingsInput = Pick<Settings, "theme" | "language" | "notifyUrls" | "notifyDays"> & {
  paperlessToken?: string;
  paperlessBacklinks?: boolean;
};

export interface User {
  id: number;
  email: string;
  name: string;
  picture: string;
  isAdmin: boolean;
}
export interface AuthInfo {
  providerName: string;
  oidc: boolean;
  devLogin: boolean;
  autoRedirect: boolean;
}

/** Server messages come with a translation key; untranslated ones as plain text. */
export const translateServer = (message: string, code?: string, params?: Record<string, unknown>): string =>
  code && i18n.exists(code) ? (i18n.t as (key: string, options?: object) => string)(code, params) : message;

/** A user-facing message for anything thrown. */
export const errorMessage = (e: unknown): string => {
  if (e instanceof ApiError) return translateServer(e.message, e.code, e.params);
  return e instanceof Error ? e.message : String(e);
};

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string,
    public params?: Record<string, unknown>,
  ) {
    super(message);
  }
}

interface ErrorBody {
  error: string;
  code?: string;
  params?: Record<string, unknown>;
}

const isErrorBody = (data: unknown): data is ErrorBody =>
  typeof data === "object" && data !== null && "error" in data && typeof data.error === "string";

const request = async <T>(method: string, url: string, body?: unknown): Promise<T> => {
  const init: RequestInit = { method, headers: {}, credentials: "same-origin" };
  if (body instanceof FormData) {
    init.body = body;
  } else if (body instanceof Blob) {
    init.body = body;
    (init.headers as Record<string, string>)["Content-Type"] = body.type || "application/octet-stream";
  } else if (body !== undefined) {
    init.body = JSON.stringify(body);
    (init.headers as Record<string, string>)["Content-Type"] = "application/json";
  }
  let res: Response;
  try {
    res = await fetch(url, init);
  } catch {
    throw new ApiError(0, i18n.t("errors.offline"));
  }
  if (res.status === 401 && !url.startsWith("/api/auth/")) {
    const ret = encodeURIComponent(location.pathname + location.search);
    if (!location.pathname.startsWith("/login")) location.assign(`/login?return=${ret}`);
    throw new ApiError(401, i18n.t("errors.signedOut"));
  }
  if (!res.ok) {
    let body: ErrorBody = { error: res.statusText || i18n.t("errors.requestFailed") };
    try {
      const data: unknown = await res.json();
      if (isErrorBody(data)) body = data;
    } catch {
      /* not JSON */
    }
    throw new ApiError(res.status, body.error, body.code, body.params);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
};

/** What restoring a backup brought back. */
export interface RestoreResult {
  householdId: number;
  name: string;
  things: number;
  photos: number;
  documents: number;
  /** Labels left on things of another household here, e.g. the one the backup was made of. */
  labelsSkipped: number;
}

export const api = {
  authInfo: () => request<AuthInfo>("GET", "/api/auth/info"),
  me: () => request<User>("GET", "/api/me"),
  logout: () => request<{ redirect: string }>("POST", "/auth/logout"),

  settings: () => request<Settings>("GET", "/api/settings"),
  saveSettings: (s: SettingsInput) => request<Settings>("PUT", "/api/settings", s),
  testNotification: (notifyUrls: string) =>
    request<{ ok: boolean }>("POST", "/api/settings/test-notification", { notifyUrls }),
  testPaperless: (token?: string) =>
    request<{ version: string; documents: number }>("POST", "/api/settings/test-paperless", { token }),

  households: () => request<{ current: number; households: Household[] }>("GET", "/api/households"),
  createHousehold: (name: string, currency: string) =>
    request<undefined>("POST", "/api/households", { name, currency }),
  switchHousehold: (id: number) => request<undefined>("POST", `/api/households/${id}/switch`),
  /** Makes a new household from a backup made with export.zip, and switches to it. */
  restore: (backup: File) => request<RestoreResult>("POST", "/api/restore", backup),
  household: () => request<Household & { members: Member[] }>("GET", "/api/household"),
  updateHousehold: (name: string, currency: string) => request<undefined>("PUT", "/api/household", { name, currency }),
  invite: () => request<{ url: string; expires: string }>("POST", "/api/household/invites"),
  setRole: (userId: number, role: Member["role"]) =>
    request<undefined>("PUT", `/api/household/members/${userId}`, { role }),
  removeMember: (userId: number) => request<undefined>("DELETE", `/api/household/members/${userId}`),
  inviteInfo: (token: string) =>
    request<{ name: string; members: number; member: boolean }>("GET", `/api/invites/${encodeURIComponent(token)}`),
  acceptInvite: (token: string) => request<undefined>("POST", `/api/invites/${encodeURIComponent(token)}/accept`),

  home: () => request<Home>("GET", "/api/home"),
  report: (inside?: number) => request<Report>("GET", `/api/report${inside ? `?in=${inside}` : ""}`),
  insured: () => request<Thing[]>("GET", "/api/report/insured"),
  search: (q: string, archived = false) =>
    request<Thing[]>("GET", `/api/search?q=${encodeURIComponent(q)}${archived ? "&archived=1" : ""}`),
  /** Moves things into another; null is into nothing. */
  move: (into: number | null, things: number[]) => request<undefined>("POST", "/api/move", { into, things }),

  things: (
    filter: {
      in?: number;
      inside?: boolean;
      top?: boolean;
      unplaced?: boolean;
      places?: boolean;
      archived?: boolean;
      templates?: boolean;
      tag?: number;
    } = {},
  ) => {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(filter)) {
      if (v === true) q.set(k, "1");
      else if (typeof v === "number") q.set(k, String(v));
    }
    return request<Thing[]>("GET", `/api/things?${q.toString()}`);
  },
  thing: (id: number) => request<ThingDetail>("GET", `/api/things/${id}`),
  createThing: (t: ThingInput) => request<Thing>("POST", "/api/things", t),
  updateThing: (id: number, t: ThingInput) => request<Thing>("PUT", `/api/things/${id}`, t),
  deleteThing: (id: number) => request<undefined>("DELETE", `/api/things/${id}`),
  archive: (id: number, archived: boolean) =>
    request<Thing>("POST", `/api/things/${id}/${archived ? "archive" : "unarchive"}`),

  /** Copies a thing, makes a template of it, or a thing from a template. */
  copyThing: (
    id: number,
    how: { name?: string; parentId?: number | null; sameParent?: boolean; template?: boolean; photos?: boolean },
  ) => request<Thing>("POST", `/api/things/${id}/copy`, how),
  templates: () => request<Thing[]>("GET", "/api/things?templates=1"),

  kinds: () => request<Kind[]>("GET", "/api/kinds"),
  createKind: (k: KindInput) => request<Kind>("POST", "/api/kinds", k),
  updateKind: (id: number, k: KindInput) => request<Kind>("PUT", `/api/kinds/${id}`, k),
  deleteKind: (id: number) => request<undefined>("DELETE", `/api/kinds/${id}`),
  icons: () => request<CustomIcon[]>("GET", "/api/icons"),
  uploadIcon: (file: File) => {
    const fd = new FormData();
    fd.append("file", file);
    return request<CustomIcon>("POST", "/api/icons", fd);
  },
  deleteIcon: (id: number) => request<undefined>("DELETE", `/api/icons/${id}`),

  uploadPhotos: (thingId: number, files: File[]) => {
    const fd = new FormData();
    for (const f of files) fd.append("file", f);
    return request<Photo[]>("POST", `/api/things/${thingId}/photos`, fd);
  },
  makeCover: (id: number) => request<undefined>("POST", `/api/photos/${id}/cover`),
  deletePhoto: (id: number) => request<undefined>("DELETE", `/api/photos/${id}`),

  label: (code: string) => request<LabelInfo>("GET", `/api/labels/${encodeURIComponent(code)}`),
  /** Puts a label on a thing; without a code it keeps its label or gets a new one. */
  attachLabel: (thingId: number, code = "") => request<{ code: string }>("POST", "/api/labels", { code, thingId }),
  detachLabel: (code: string) => request<undefined>("DELETE", `/api/labels/${encodeURIComponent(code)}`),
  blankLabels: (count: number) => request<{ codes: string[] }>("POST", "/api/labels/blank", { count }),

  tags: () => request<Tag[]>("GET", "/api/tags"),
  tag: (id: number) => request<Tag & { things: Thing[] }>("GET", `/api/tags/${id}`),
  updateTag: (id: number, name: string, color: TagColor) =>
    request<undefined>("PATCH", `/api/tags/${id}`, { name, color }),
  deleteTag: (id: number) => request<undefined>("DELETE", `/api/tags/${id}`),

  fields: () => request<Field[]>("GET", "/api/fields"),
  createField: (name: string, type: FieldType) => request<Field>("POST", "/api/fields", { name, type }),
  updateField: (id: number, name: string, position: number) =>
    request<undefined>("PUT", `/api/fields/${id}`, { name, position }),
  deleteField: (id: number) => request<undefined>("DELETE", `/api/fields/${id}`),

  uploadAttachment: (thingId: number, file: File, kind: AttachmentKind) => {
    const fd = new FormData();
    fd.append("kind", kind);
    fd.append("file", file);
    return request<Attachment>("POST", `/api/things/${thingId}/attachments`, fd);
  },
  linkPaperless: (thingId: number, documentIds: number[], kind: AttachmentKind) =>
    request<Attachment[]>("POST", `/api/things/${thingId}/attachments/paperless`, { documentIds, kind }),
  searchPaperless: (q: string) => request<PaperlessDoc[]>("GET", `/api/paperless/search?q=${encodeURIComponent(q)}`),
  updateAttachment: (id: number, kind: AttachmentKind, filename: string) =>
    request<Attachment>("PATCH", `/api/attachments/${id}`, { kind, filename }),
  deleteAttachment: (id: number) => request<undefined>("DELETE", `/api/attachments/${id}`),

  dueMaintenance: (days = 30) => request<Maintenance[]>("GET", `/api/maintenance/due?days=${days}`),
  addMaintenance: (thingId: number, m: MaintenanceInput) =>
    request<Maintenance>("POST", `/api/things/${thingId}/maintenance`, m),
  updateMaintenance: (id: number, m: MaintenanceInput) => request<Maintenance>("PUT", `/api/maintenance/${id}`, m),
  deleteMaintenance: (id: number) => request<undefined>("DELETE", `/api/maintenance/${id}`),

  addLink: (thingId: number, url: string, title: string) =>
    request<Link>("POST", `/api/things/${thingId}/links`, { url, title }),
  deleteLink: (id: number) => request<undefined>("DELETE", `/api/links/${id}`),

  tokens: () => request<ApiToken[]>("GET", "/api/tokens"),
  /** The token itself is in the answer, and only there. */
  createToken: (name: string) => request<ApiToken & { token: string }>("POST", "/api/tokens", { name }),
  deleteToken: (id: number) => request<undefined>("DELETE", `/api/tokens/${id}`),

  /** Allows another app; answers where to send the browser. */
  allowConnect: (app: string, redirectUri: string, state: string, household: number) =>
    request<{ redirect: string }>("POST", "/api/connect", { app, redirectUri, state, household }),
  documentThings: (doc: number) => request<Thing[]>("GET", `/api/paperless/documents/${doc}/things`),

  importHomebox: (url: string, token: string) => request<ImportProgress>("POST", "/api/import/homebox", { url, token }),
  importStatus: () => request<ImportProgress | null>("GET", "/api/import/homebox"),
};

export const attachmentUrl = (a: Attachment, download = false) =>
  `/api/attachments/${a.id}/file${download ? "?download=1" : ""}`;

export const photoUrl = (id: number, thumb = false) => `/api/photos/${id}${thumb ? "?size=thumb" : ""}`;

/** "House › Kitchen › Drawer". */
export const pathText = (path: Ref[]) => path.map((p) => p.name).join(" › ");

/** Where a custom icon is served, for an icon value "custom:<id>". */
export const customIconUrl = (icon: string) => `/api/icons/${icon.slice("custom:".length)}`;

/** A thing as input, for saving it back changed. */
export const toInput = (t: Thing): ThingInput & { tags: string[]; fields: Record<number, string> } => ({
  parentId: t.parentId,
  kindId: t.kind.id,
  name: t.name,
  description: t.description,
  quantity: t.quantity,
  manufacturer: t.manufacturer,
  model: t.model,
  serial: t.serial,
  notes: t.notes,
  purchaseDate: t.purchaseDate,
  purchasePrice: t.purchasePrice,
  purchaseFrom: t.purchaseFrom,
  currency: t.currency,
  warrantyUntil: t.warrantyUntil,
  lifetimeWarranty: t.lifetimeWarranty,
  warrantyDetails: t.warrantyDetails,
  insured: t.insured,
  soldDate: t.soldDate,
  soldPrice: t.soldPrice,
  soldTo: t.soldTo,
  soldNotes: t.soldNotes,
  tags: t.tags.map((tag) => tag.name),
  fields: Object.fromEntries((t.fields ?? []).map((f) => [f.fieldId, f.value])),
});
