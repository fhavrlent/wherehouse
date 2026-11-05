import {
  Bell,
  Copy,
  Download,
  FileDown,
  FileText,
  Home as HomeIcon,
  KeyRound,
  LayoutTemplate,
  ListPlus,
  Palette,
  Shapes,
  Trash2,
  UserPlus,
} from "lucide-react";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { Card, Segmented } from "@/components/ui/misc";
import { browserLanguage, LANGUAGES } from "@/i18n";
import { api, errorMessage, FIELD_TYPES, type FieldType, type Kind, type Settings as SettingsT } from "@/lib/api";
import { kindName } from "@/lib/kinds";
import { IconPicker } from "@/components/IconPicker";
import { KindIcon } from "@/components/KindIcon";
import { Switch } from "@/components/ui/misc";
import {
  keys,
  useFields,
  useHousehold,
  useInvalidate,
  useKinds,
  useMe,
  useSaveSettings,
  useSettingsQuery,
  useTemplates,
} from "@/lib/queries";
import { Link, useNavigate } from "react-router";
import { linkButton } from "@/components/ui/buttonStyles";
import { cn } from "@/lib/utils";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { applyTheme } from "@/lib/theme";

export const Settings = () => {
  const { t } = useTranslation();
  const { data: settings, isLoading } = useSettingsQuery();
  if (isLoading || !settings) return <div className="h-96 animate-pulse rounded-2xl bg-surface-3/60" />;
  return (
    <div className="animate-fade-in">
      <h1 className="text-2xl font-semibold tracking-tight">{t("settings.title")}</h1>
      <p className="mt-1 mb-8 text-sm text-fg-muted">{t("settings.autosave")}</p>
      <div className="flex flex-col gap-6">
        <Display s={settings} />
        <HouseholdSection />
        <Kinds />
        <Fields />
        <Reminders s={settings} />
        {settings.paperlessUrl && <Paperless s={settings} />}
        <Templates />
        <ExportSection />
        <Tokens />
        {settings.isAdmin && <HomeBoxImport />}
      </div>
    </div>
  );
};

const Section = ({
  icon,
  title,
  description,
  children,
}: {
  icon: ReactNode;
  title: string;
  description: ReactNode;
  children: ReactNode;
}) => (
  <Card className="grid gap-6 p-5 sm:p-6 md:grid-cols-[16rem_1fr]">
    <div>
      <h2 className="flex items-center gap-2 text-sm font-semibold [&_svg]:size-4 [&_svg]:text-fg-subtle">
        {icon}
        {title}
      </h2>
      <p className="mt-1.5 text-[13px] leading-relaxed text-fg-muted">{description}</p>
    </div>
    <div className="flex min-w-0 flex-col gap-5">{children}</div>
  </Card>
);

/** Saves a partial change immediately and reports errors. */
const useAutoSave = (s: SettingsT) => {
  const save = useSaveSettings();
  return async (patch: Partial<SettingsT> & { paperlessToken?: string }) => {
    try {
      await save.mutateAsync({ ...s, ...patch });
      return true;
    } catch (e) {
      toast.error(errorMessage(e));
      return false;
    }
  };
};

const Display = ({ s }: { s: SettingsT }) => {
  const { t } = useTranslation();
  const update = useAutoSave(s);
  return (
    <Section icon={<Palette />} title={t("settings.display.title")} description={t("settings.display.description")}>
      <Field label={t("settings.display.language")}>
        <Select value={s.language} onChange={(e) => update({ language: e.target.value })} className="sm:w-72">
          <option value="">
            {t("settings.display.languageAuto", { language: LANGUAGES[browserLanguage()]?.name ?? "English" })}
          </option>
          {s.languages
            .filter((code) => code in LANGUAGES)
            .map((code) => (
              <option key={code} value={code}>
                {LANGUAGES[code]?.name ?? code}
              </option>
            ))}
        </Select>
      </Field>
      <Field label={t("settings.display.theme")}>
        <Segmented
          value={s.theme}
          onChange={(theme) => {
            applyTheme(theme);
            void update({ theme });
          }}
          options={[
            { value: "system", label: t("theme.system") },
            { value: "light", label: t("theme.light") },
            { value: "dark", label: t("theme.dark") },
          ]}
        />
      </Field>
    </Section>
  );
};

const Reminders = ({ s }: { s: SettingsT }) => {
  const { t } = useTranslation();
  const update = useAutoSave(s);
  const [urls, setUrls] = useState(s.notifyUrls);
  const [days, setDays] = useState(s.notifyDays);
  const [testing, setTesting] = useState(false);
  const test = async () => {
    setTesting(true);
    try {
      await api.testNotification(urls);
      toast.success(t("settings.reminders.sent"));
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setTesting(false);
    }
  };
  return (
    <Section icon={<Bell />} title={t("settings.reminders.title")} description={t("settings.reminders.description")}>
      <Field
        label={t("settings.reminders.urls")}
        hint={
          s.isAdmin || s.notifyAllowedHosts.length === 0
            ? t("settings.reminders.urlsHint")
            : t("settings.reminders.urlsHintHosts", { hosts: s.notifyAllowedHosts.join(", ") })
        }
      >
        <Textarea
          value={urls}
          onChange={(e) => setUrls(e.target.value)}
          onBlur={() => urls !== s.notifyUrls && void update({ notifyUrls: urls })}
          placeholder="ntfy://ntfy.sh/my-topic"
          className="font-mono text-[13px]"
        />
      </Field>
      <div className="flex flex-wrap items-end gap-3">
        <Field label={t("settings.reminders.days")} hint={t("settings.reminders.daysHint")}>
          <Input
            value={days}
            onChange={(e) => setDays(e.target.value)}
            onBlur={() =>
              days !== s.notifyDays &&
              void update({ notifyDays: days }).then((ok) => {
                if (!ok) setDays(s.notifyDays);
              })
            }
            className="w-40 tabular-nums"
          />
        </Field>
        <Button variant="outline" onClick={test} loading={testing} disabled={!urls.trim()}>
          {t("settings.reminders.test")}
        </Button>
      </div>
    </Section>
  );
};

const Paperless = ({ s }: { s: SettingsT }) => {
  const { t } = useTranslation();
  const update = useAutoSave(s);
  const [token, setToken] = useState("");
  const [testing, setTesting] = useState(false);
  const test = async () => {
    setTesting(true);
    try {
      const info = await api.testPaperless(token || undefined);
      toast.success(t("settings.paperless.ok", { documents: info.documents }));
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setTesting(false);
    }
  };
  return (
    <Section
      icon={<FileText />}
      title="Paperless-ngx"
      description={t("settings.paperless.description", { url: s.paperlessUrl })}
    >
      <Field
        label={t("settings.paperless.token")}
        hint={
          s.hasPaperlessToken
            ? t("settings.paperless.tokenSaved")
            : s.paperlessServerToken
              ? t("settings.paperless.serverToken")
              : t("settings.paperless.tokenHint")
        }
      >
        <div className="flex gap-2">
          <Input
            type="password"
            autoComplete="off"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder={s.hasPaperlessToken ? "••••••••" : ""}
          />
          <Button variant="outline" onClick={test} loading={testing}>
            {t("settings.paperless.test")}
          </Button>
          <Button
            disabled={!token.trim()}
            onClick={() =>
              void update({ paperlessToken: token }).then((ok) => {
                if (ok) setToken("");
              })
            }
          >
            {t("common.save")}
          </Button>
        </div>
      </Field>
      <label className="flex items-start gap-3 text-sm">
        <Switch
          checked={s.paperlessBacklinks}
          onCheckedChange={(paperlessBacklinks) => update({ paperlessBacklinks })}
        />
        <span>
          {t("settings.paperless.backlinks")}
          <span className="block text-xs text-fg-subtle">
            {t("settings.paperless.backlinksHint", { field: s.paperlessField })}
          </span>
        </span>
      </label>
      {s.hasPaperlessToken && (
        <button
          type="button"
          onClick={() => update({ paperlessToken: "" })}
          className="w-fit cursor-pointer text-sm text-fg-muted hover:text-bad"
        >
          {t("settings.paperless.forget")}
        </button>
      )}
    </Section>
  );
};

const Fields = () => {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const invalidate = useInvalidate();
  const { data: fields = [] } = useFields();
  const [name, setName] = useState("");
  const [type, setType] = useState<FieldType>("text");
  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ["fields"] });
    invalidate();
  };
  const run = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
      return true;
    } catch (e) {
      toast.error(errorMessage(e));
      return false;
    } finally {
      refresh();
    }
  };
  return (
    <Section icon={<ListPlus />} title={t("settings.fields.title")} description={t("settings.fields.description")}>
      {fields.length > 0 && (
        <ul className="flex flex-col gap-2">
          {fields.map((f) => (
            <li key={f.id} className="flex items-center gap-2">
              <Input
                defaultValue={f.name}
                onBlur={(e) =>
                  e.target.value !== f.name && void run(() => api.updateField(f.id, e.target.value, f.position))
                }
                className="flex-1"
              />
              <span className="w-24 text-xs text-fg-muted">{t(`fieldTypes.${f.type}`)}</span>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("settings.fields.delete", { name: f.name })}
                onClick={() => run(() => api.deleteField(f.id))}
              >
                <Trash2 />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void run(() => api.createField(name, type)).then((ok) => ok && setName(""));
        }}
      >
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("settings.fields.namePlaceholder")}
          className="min-w-40 flex-1"
        />
        <Select value={type} onChange={(e) => setType(e.target.value as FieldType)} className="w-36">
          {FIELD_TYPES.map((ft) => (
            <option key={ft} value={ft}>
              {t(`fieldTypes.${ft}`)}
            </option>
          ))}
        </Select>
        <Button type="submit" variant="outline" disabled={!name.trim()}>
          {t("settings.fields.add")}
        </Button>
      </form>
    </Section>
  );
};

const HomeBoxImport = () => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const [url, setUrl] = useState("");
  const [token, setToken] = useState("");
  const [starting, setStarting] = useState(false);
  const status = useQuery({
    queryKey: ["import"],
    queryFn: api.importStatus,
    refetchInterval: (q) => (q.state.data?.running ? 1500 : false),
  });
  const p = status.data;
  // Show what was imported as soon as the import ends.
  const [wasRunning, setWasRunning] = useState(false);
  if ((p?.running ?? false) !== wasRunning) {
    setWasRunning(p?.running ?? false);
    if (!p?.running) invalidate();
  }
  const start = async () => {
    setStarting(true);
    try {
      await api.importHomebox(url, token);
      setToken("");
      await status.refetch();
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setStarting(false);
    }
  };
  return (
    <Section icon={<Download />} title={t("settings.import.title")} description={t("settings.import.description")}>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label={t("settings.import.url")}>
          <Input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://homebox.example.com" />
        </Field>
        <Field label={t("settings.import.token")} hint={t("settings.import.tokenHint")}>
          <Input type="password" autoComplete="off" value={token} onChange={(e) => setToken(e.target.value)} />
        </Field>
      </div>
      <Button
        onClick={start}
        loading={starting || p?.running}
        disabled={!url.trim() || !token.trim() || p?.running}
        className="w-fit"
      >
        {p?.running ? t("settings.import.running") : t("settings.import.start")}
      </Button>
      {p && (
        <div className="flex flex-col gap-2 rounded-xl bg-surface-2 p-4 text-sm">
          {p.total > 0 && (
            <div className="h-1.5 overflow-hidden rounded-full bg-surface-3">
              <div className="h-full bg-primary transition-all" style={{ width: `${(100 * p.done) / p.total}%` }} />
            </div>
          )}
          <p className="text-fg-muted">
            {t("settings.import.counts", {
              things: p.things,
              photos: p.photos,
              documents: p.documents,
              links: p.links,
              maintenance: p.maintenance,
              skipped: p.skipped,
            })}
          </p>
          {p.error && <p className="text-bad">{p.error}</p>}
          {p.errors.length > 0 && (
            <details>
              <summary className="cursor-pointer text-fg-muted">
                {t("settings.import.problems", { count: p.errors.length })}
              </summary>
              <ul className="mt-2 list-disc pl-5 text-xs text-fg-muted">
                {p.errors.map((e) => (
                  <li key={e}>{e}</li>
                ))}
              </ul>
            </details>
          )}
        </div>
      )}
    </Section>
  );
};

const Tokens = () => {
  const { t } = useTranslation();
  const { data: tokens = [], refetch } = useQuery({ queryKey: ["tokens"], queryFn: api.tokens });
  const [name, setName] = useState("");
  const [shown, setShown] = useState<string | null>(null);
  const create = async () => {
    try {
      const made = await api.createToken(name);
      setShown(made.token);
      setName("");
    } catch (e) {
      toast.error(errorMessage(e));
    }
    void refetch();
  };
  const remove = async (id: number) => {
    try {
      await api.deleteToken(id);
    } catch (e) {
      toast.error(errorMessage(e));
    }
    void refetch();
  };
  return (
    <Section icon={<KeyRound />} title={t("settings.tokens.title")} description={t("settings.tokens.description")}>
      {shown && (
        <div className="flex flex-col gap-2 rounded-xl border border-primary/40 bg-primary-soft/40 p-3">
          <p className="text-xs text-fg-muted">{t("settings.tokens.copyNow")}</p>
          <div className="flex gap-2">
            <code className="min-w-0 flex-1 truncate rounded-lg bg-surface px-3 py-2 font-mono text-xs">{shown}</code>
            <Button
              variant="outline"
              size="icon"
              aria-label={t("settings.tokens.copy")}
              onClick={() =>
                navigator.clipboard.writeText(shown).then(
                  () => toast.success(t("settings.tokens.copied")),
                  () => toast.error(t("settings.tokens.copyFailed")),
                )
              }
            >
              <Copy />
            </Button>
          </div>
        </div>
      )}
      {tokens.length > 0 && (
        <ul className="divide-y divide-border rounded-xl border border-border">
          {tokens.map((tok) => (
            <li key={tok.id} className="flex items-center gap-3 px-3 py-2 text-sm">
              <span className="min-w-0 flex-1 truncate font-medium">{tok.name}</span>
              <span className="text-xs text-fg-subtle">
                {tok.lastUsedAt
                  ? t("settings.tokens.used", { date: tok.lastUsedAt.slice(0, 10) })
                  : t("settings.tokens.unused")}
              </span>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("settings.tokens.delete", { name: tok.name })}
                onClick={() => remove(tok.id)}
              >
                <Trash2 />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void create();
        }}
      >
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("settings.tokens.namePlaceholder")}
        />
        <Button type="submit" variant="outline" disabled={!name.trim()}>
          {t("settings.tokens.create")}
        </Button>
      </form>
    </Section>
  );
};

const HouseholdSection = () => {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { data: h } = useHousehold();
  const { data: me } = useMe();
  const [invite, setInvite] = useState<string | null>(null);
  if (!h) return null;
  const owner = h.role === "owner";
  const refresh = () => {
    void qc.invalidateQueries({ queryKey: keys.household });
    void qc.invalidateQueries({ queryKey: keys.households });
  };
  const run = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
    } catch (e) {
      toast.error(errorMessage(e));
    }
    refresh();
  };
  return (
    <Section
      icon={<HomeIcon />}
      title={t("settings.household.title")}
      description={t("settings.household.description")}
    >
      <div className="grid gap-4 sm:grid-cols-[1fr_7rem]">
        <Field label={t("settings.household.name")}>
          <Input
            key={h.id}
            defaultValue={h.name}
            disabled={!owner}
            onBlur={(e) => e.target.value !== h.name && void run(() => api.updateHousehold(e.target.value, h.currency))}
          />
        </Field>
        <Field label={t("settings.household.currency")}>
          <Input
            key={h.id}
            defaultValue={h.currency}
            disabled={!owner}
            maxLength={3}
            className="uppercase"
            onBlur={(e) =>
              e.target.value.toUpperCase() !== h.currency &&
              void run(() => api.updateHousehold(h.name, e.target.value.toUpperCase()))
            }
          />
        </Field>
      </div>
      <Field label={t("settings.household.members")}>
        <ul className="divide-y divide-border rounded-xl border border-border">
          {h.members.map((m) => (
            <li key={m.userId} className="flex items-center gap-3 px-3 py-2 text-sm">
              <span className="min-w-0 flex-1">
                <span className="block truncate font-medium">{m.name || m.email}</span>
                <span className="block truncate text-xs text-fg-subtle">{m.email}</span>
              </span>
              {owner && m.userId !== me?.id ? (
                <Select
                  value={m.role}
                  onChange={(e) => run(() => api.setRole(m.userId, e.target.value as "owner" | "member"))}
                  className="h-7 w-28 text-xs"
                >
                  <option value="owner">{t("settings.household.owner")}</option>
                  <option value="member">{t("settings.household.member")}</option>
                </Select>
              ) : (
                <span className="text-xs text-fg-muted">
                  {m.role === "owner" ? t("settings.household.owner") : t("settings.household.member")}
                </span>
              )}
              {(owner || m.userId === me?.id) && (
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={m.userId === me?.id ? t("settings.household.leave") : t("settings.household.remove")}
                  onClick={() =>
                    run(async () => {
                      await api.removeMember(m.userId);
                      if (m.userId === me?.id) await qc.invalidateQueries();
                    })
                  }
                >
                  <Trash2 />
                </Button>
              )}
            </li>
          ))}
        </ul>
      </Field>
      {owner && (
        <div className="flex flex-col gap-2">
          <Button
            variant="outline"
            className="w-fit"
            onClick={() =>
              run(async () => {
                setInvite((await api.invite()).url);
              })
            }
          >
            <UserPlus /> {t("settings.household.invite")}
          </Button>
          {invite && (
            <div className="flex flex-col gap-1.5">
              <div className="flex gap-2">
                <code className="min-w-0 flex-1 truncate rounded-lg bg-surface-2 px-3 py-2 font-mono text-xs">
                  {invite}
                </code>
                <Button
                  variant="outline"
                  size="icon"
                  aria-label={t("settings.tokens.copy")}
                  onClick={() =>
                    navigator.clipboard.writeText(invite).then(
                      () => toast.success(t("settings.tokens.copied")),
                      () => toast.error(t("settings.tokens.copyFailed")),
                    )
                  }
                >
                  <Copy />
                </Button>
              </div>
              <p className="text-xs text-fg-subtle">{t("settings.household.inviteHint")}</p>
            </div>
          )}
        </div>
      )}
    </Section>
  );
};

const Kinds = () => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const { data: kinds = [] } = useKinds();
  const [picking, setPicking] = useState<Kind | "new" | null>(null);
  const [newKind, setNewKind] = useState({ name: "", icon: "shapes", isPlace: false });
  const run = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
      return true;
    } catch (e) {
      toast.error(errorMessage(e));
      return false;
    } finally {
      invalidate();
    }
  };
  const update = (k: Kind, patch: Partial<Kind>) => {
    const next = { ...k, ...patch };
    return run(() =>
      api.updateKind(k.id, { name: next.name, icon: next.icon, isPlace: next.isPlace, position: next.position }),
    );
  };
  return (
    <Section icon={<Shapes />} title={t("settings.kinds.title")} description={t("settings.kinds.description")}>
      <ul className="flex flex-col gap-2">
        {kinds.map((k) => (
          <li key={k.id} className="flex items-center gap-2">
            <button
              type="button"
              aria-label={t("settings.kinds.icon")}
              onClick={() => setPicking(k)}
              className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-lg border border-border text-fg-muted hover:border-border-strong [&>*]:size-5"
            >
              <KindIcon icon={k.icon} />
            </button>
            <Input
              key={`${k.id}-${k.name}`}
              defaultValue={kindName(k, t)}
              onBlur={(e) => e.target.value !== kindName(k, t) && void update(k, { name: e.target.value })}
              className="min-w-0 flex-1"
            />
            <label
              className="flex shrink-0 items-center gap-2 text-xs text-fg-muted"
              title={t("settings.kinds.placeHint")}
            >
              <Switch
                checked={k.isPlace}
                disabled={k.key === "thing"}
                onCheckedChange={(isPlace) => update(k, { isPlace })}
              />
              <span className="hidden sm:inline">{t("settings.kinds.place")}</span>
            </label>
            <span className="w-10 text-right text-xs text-fg-subtle tabular-nums">{k.count}</span>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("settings.kinds.delete", { name: kindName(k, t) })}
              disabled={k.count > 0 || k.key === "thing"}
              onClick={() => run(() => api.deleteKind(k.id))}
            >
              <Trash2 />
            </Button>
          </li>
        ))}
      </ul>
      <form
        className="flex items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void run(() => api.createKind({ ...newKind, position: 0 })).then(
            (ok) => ok && setNewKind({ name: "", icon: "shapes", isPlace: false }),
          );
        }}
      >
        <button
          type="button"
          aria-label={t("settings.kinds.icon")}
          onClick={() => setPicking("new")}
          className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-lg border border-dashed border-border-strong text-fg-muted [&>*]:size-5"
        >
          <KindIcon icon={newKind.icon} />
        </button>
        <Input
          value={newKind.name}
          onChange={(e) => setNewKind({ ...newKind, name: e.target.value })}
          placeholder={t("settings.kinds.namePlaceholder")}
          className="min-w-0 flex-1"
        />
        <label className="flex shrink-0 items-center gap-2 text-xs text-fg-muted">
          <Switch checked={newKind.isPlace} onCheckedChange={(isPlace) => setNewKind({ ...newKind, isPlace })} />
          <span className="hidden sm:inline">{t("settings.kinds.place")}</span>
        </label>
        <Button type="submit" variant="outline" disabled={!newKind.name.trim()}>
          {t("settings.kinds.add")}
        </Button>
      </form>
      <IconPicker
        open={picking !== null}
        onOpenChange={(o) => !o && setPicking(null)}
        value={picking === "new" ? newKind.icon : (picking?.icon ?? "")}
        onPick={(icon) => {
          if (picking === "new") setNewKind({ ...newKind, icon });
          else if (picking) void update(picking, { icon });
        }}
      />
    </Section>
  );
};

const Templates = () => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const { data: templates = [] } = useTemplates();
  return (
    <Section
      icon={<LayoutTemplate />}
      title={t("settings.templates.title")}
      description={t("settings.templates.description")}
    >
      {templates.length === 0 ? (
        <p className="text-sm text-fg-subtle">{t("settings.templates.none")}</p>
      ) : (
        <ul className="divide-y divide-border rounded-xl border border-border">
          {templates.map((tpl) => (
            <li key={tpl.id} className="flex items-center gap-3 px-3 py-2 text-sm">
              <KindIcon icon={tpl.kind.icon} className="size-4 text-fg-subtle" />
              <Link to={`/i/${tpl.id}`} className="min-w-0 flex-1 truncate hover:text-primary">
                {tpl.name}
              </Link>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("settings.templates.delete", { name: tpl.name })}
                onClick={async () => {
                  try {
                    await api.deleteThing(tpl.id);
                  } catch (e) {
                    toast.error(errorMessage(e));
                  }
                  invalidate();
                }}
              >
                <Trash2 />
              </Button>
            </li>
          ))}
        </ul>
      )}
    </Section>
  );
};

const ExportSection = () => {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [restoring, setRestoring] = useState(false);
  // A backup comes back as a household of its own, so nothing here changes.
  const restore = async (file: File) => {
    setRestoring(true);
    try {
      const res = await api.restore(file);
      await qc.invalidateQueries();
      toast.success(t("settings.export.restored", { name: res.name, count: res.things }), {
        description: res.labelsSkipped ? t("settings.export.labelsSkipped", { count: res.labelsSkipped }) : undefined,
      });
      void navigate("/");
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setRestoring(false);
    }
  };
  return (
    <Section icon={<FileDown />} title={t("settings.export.title")} description={t("settings.export.description")}>
      <div className="flex flex-wrap gap-2">
        <a href="/api/export.csv" className={linkButton("outline")}>
          {t("settings.export.csv")}
        </a>
        <a href="/api/export.zip" className={linkButton("outline")}>
          {t("settings.export.zip")}
        </a>
      </div>
      <p className="text-xs text-fg-subtle">{t("settings.export.hint")}</p>
      <div className="flex flex-col gap-1.5">
        <label
          className={cn(linkButton("outline"), "w-fit cursor-pointer", restoring && "pointer-events-none opacity-60")}
        >
          {restoring ? t("settings.export.restoring") : t("settings.export.restore")}
          <input
            type="file"
            accept=".zip,application/zip"
            className="sr-only"
            disabled={restoring}
            onChange={(e) => {
              const file = e.target.files?.[0];
              e.target.value = "";
              if (file) void restore(file);
            }}
          />
        </label>
        <p className="text-xs text-fg-subtle">{t("settings.export.restoreHint")}</p>
      </div>
    </Section>
  );
};
