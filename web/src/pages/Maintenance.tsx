import { Search, Wrench } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { DueList } from "@/components/DueList";
import { DoneDialog } from "@/components/MaintenanceDone";
import { Section } from "@/components/Things";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/misc";
import { pathText, type Maintenance } from "@/lib/api";
import { daysUntil } from "@/lib/dates";
import { useHousehold, useMaintenance } from "@/lib/queries";
import { formatMoney } from "@/lib/utils";

/** Everything scheduled in the household, by when it's due, and what was done lately. */
export const MaintenancePage = () => {
  const { t } = useTranslation();
  const { data = [], isLoading } = useMaintenance();
  const { data: household } = useHousehold();
  const [filter, setFilter] = useState("");
  const [finishing, setFinishing] = useState<Maintenance | null>(null);

  if (isLoading) return <div className="h-96 animate-pulse rounded-2xl bg-surface-3/60" />;

  const q = filter.trim().toLocaleLowerCase();
  const shown = q
    ? data.filter((m) =>
        [m.title, m.thingName ?? "", m.path ? pathText(m.path) : ""].some((s) => s.toLocaleLowerCase().includes(q)),
      )
    : data;
  const left = (m: Maintenance) => (m.nextDue ? daysUntil(m.nextDue) : null);
  const within = (from: number, to: number) =>
    shown.filter((m) => {
      const n = left(m);
      return n !== null && n >= from && n <= to;
    });
  const overdue = within(-Infinity, -1);
  const soon = within(0, 30);
  const later = within(31, Infinity);
  const done = shown.filter((m) => !m.nextDue && m.doneOn);
  const spent = done.reduce((sum, m) => sum + (m.cost ?? 0), 0);
  const currency = household?.currency ?? "EUR";

  return (
    <div className="animate-fade-in flex flex-col gap-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold tracking-tight">{t("maintenancePage.title")}</h1>
        {data.length > 0 && (
          <div className="relative w-full sm:w-72">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-fg-subtle" />
            <Input
              type="search"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder={t("maintenancePage.filter")}
              aria-label={t("maintenancePage.filter")}
              className="pl-9"
            />
          </div>
        )}
      </div>

      {data.length === 0 ? (
        <Card className="flex flex-col items-center px-6 py-16 text-center">
          <span className="flex size-12 items-center justify-center rounded-2xl bg-primary-soft text-primary">
            <Wrench className="size-6" />
          </span>
          <p className="mt-4 max-w-sm text-sm text-fg-muted">{t("maintenancePage.empty")}</p>
        </Card>
      ) : shown.length === 0 ? (
        <p className="text-sm text-fg-subtle">{t("maintenancePage.noMatch")}</p>
      ) : (
        <>
          {overdue.length > 0 && (
            <Section title={t("maintenancePage.overdue")}>
              <DueList entries={overdue} onDone={setFinishing} />
            </Section>
          )}
          {soon.length > 0 && (
            <Section title={t("maintenancePage.soon")}>
              <DueList entries={soon} onDone={setFinishing} />
            </Section>
          )}
          {later.length > 0 && (
            <Section title={t("maintenancePage.later")}>
              <DueList entries={later} onDone={setFinishing} />
            </Section>
          )}
          {done.length > 0 && (
            <Section
              title={t("maintenancePage.done")}
              action={
                spent > 0 && (
                  <span className="text-xs text-fg-muted tabular-nums">
                    {t("maintenancePage.doneTotal", { total: formatMoney(spent, currency) })}
                  </span>
                )
              }
            >
              <ul className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-surface shadow-card">
                {done.map((m) => (
                  <li key={m.id}>
                    <Link to={`/i/${m.thingId}`} className="flex items-center gap-3 px-3 py-2.5 hover:bg-surface-2">
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm">{m.title}</span>
                        <span className="block truncate text-xs text-fg-subtle">{m.thingName}</span>
                      </span>
                      <span className="flex shrink-0 flex-col items-end text-xs text-fg-muted tabular-nums">
                        <span>{m.doneOn}</span>
                        {m.cost !== null && <span className="text-fg-subtle">{formatMoney(m.cost, currency)}</span>}
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            </Section>
          )}
        </>
      )}
      <DoneDialog entry={finishing} onClose={() => setFinishing(null)} />
    </div>
  );
};
