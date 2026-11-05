import { useQuery } from "@tanstack/react-query";
import { Printer, ShieldCheck } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router";
import { KindIcon } from "@/components/KindIcon";
import { PlaceField } from "@/components/PlacePicker";
import { Breadcrumbs, Section, ThingList } from "@/components/Things";
import { linkButton } from "@/components/ui/buttonStyles";
import { Card, CardHeader } from "@/components/ui/misc";
import { api, type Group, type Money } from "@/lib/api";
import { kindName } from "@/lib/kinds";
import { formatMoney } from "@/lib/utils";

/** The main currency's amount, then any others. */
const Amount = ({ money, currency, className }: { money: Money; currency: string; className?: string }) => {
  const others = Object.entries(money).filter(([c, v]) => c !== currency && v > 0);
  return (
    <span className={className}>
      <span className="tabular-nums">{formatMoney(money[currency] ?? 0, currency)}</span>
      {others.map(([c, v]) => (
        <span key={c} className="ml-2 text-[0.7em] font-normal text-fg-muted tabular-nums">
          + {formatMoney(v, c)}
        </span>
      ))}
    </span>
  );
};

const Tile = ({ label, children, hint }: { label: string; children: ReactNode; hint?: string }) => (
  <Card className="p-4">
    <p className="text-xs text-fg-muted">{label}</p>
    <p className="mt-1 text-lg font-semibold tracking-tight break-words tabular-nums sm:text-xl">{children}</p>
    {hint && <p className="mt-1 text-xs text-fg-subtle">{hint}</p>}
  </Card>
);

/**
 * Value by place, tag or kind, as bars in one hue: one series, so no
 * legend; the value is written at each bar's end, the count in the tooltip.
 */
const Bars = ({
  groups,
  currency,
  label,
  link,
}: {
  groups: Group[];
  currency: string;
  label: (g: Group) => ReactNode;
  link?: (g: Group) => string;
}) => {
  const { t } = useTranslation();
  const max = Math.max(...groups.map((g) => g.value[currency] ?? 0), 1);
  return (
    <ul className="flex flex-col gap-2.5 p-4">
      {groups.map((g) => {
        const v = g.value[currency] ?? 0;
        const name = link ? (
          <Link to={link(g)} className="flex min-w-0 items-center gap-1.5 hover:text-primary">
            {label(g)}
          </Link>
        ) : (
          label(g)
        );
        return (
          <li
            key={g.id}
            title={`${formatMoney(v, currency)} · ${t("report.count", { count: g.count })}`}
            className="grid grid-cols-[minmax(0,6.5rem)_1fr_auto] items-center gap-3 text-sm sm:grid-cols-[minmax(6rem,10rem)_1fr_auto]"
          >
            <span className="flex min-w-0 items-center gap-1.5 truncate">{name}</span>
            <span className="h-3 rounded-r bg-surface-3">
              <span
                className="block h-full rounded-r bg-primary"
                style={{ width: `${Math.max((100 * v) / max, v > 0 ? 1 : 0)}%` }}
              />
            </span>
            <Amount money={g.value} currency={currency} className="text-right text-xs text-fg-muted" />
          </li>
        );
      })}
    </ul>
  );
};

export const Report = () => {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const inParam = params.get("in");
  const inId = inParam ? Number(inParam) : null;
  const { data: r, isLoading } = useQuery({
    queryKey: ["report", inId],
    queryFn: () => api.report(inId ?? undefined),
  });
  if (isLoading || !r) return <div className="h-96 animate-pulse rounded-2xl bg-surface-3/60" />;
  const cur = r.currency;

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{t("report.title")}</h1>
          <p className="mt-1 text-sm text-fg-muted">{t("report.description")}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="w-64">
            <PlaceField
              value={inId}
              onChange={(id) => setParams(id ? { in: String(id) } : {})}
              noneLabel={t("report.everything")}
              title={t("report.scope")}
            />
          </div>
          <Link to="/report/insurance" className={linkButton("outline")}>
            <ShieldCheck /> {t("report.insuranceList")}
          </Link>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Tile label={t("report.value")} hint={t("report.valueHint")}>
          <Amount money={r.value} currency={cur} />
        </Tile>
        <Tile label={t("report.insured")}>
          <Amount money={r.insured} currency={cur} />
        </Tile>
        <Tile
          label={t("report.things")}
          hint={r.things > r.priced ? t("report.unpriced", { count: r.things - r.priced }) : undefined}
        >
          <span className="tabular-nums">{r.things}</span>
        </Tile>
        <Tile label={t("report.sold")}>
          <Amount money={r.sold} currency={cur} />
        </Tile>
      </div>

      <div className="grid gap-4 lg:grid-cols-2 lg:[&>:last-child:nth-child(odd)]:col-span-2">
        {r.byPlace.length > 0 && (
          <Card className="overflow-hidden">
            <CardHeader title={t("report.byPlace")} />
            <Bars
              groups={r.byPlace}
              currency={cur}
              link={(g) => `/report?in=${g.id}`}
              label={(g) => (
                <>
                  <KindIcon icon={g.icon ?? "shapes"} className="size-4 shrink-0 text-fg-subtle" />
                  <span className="truncate">{g.name}</span>
                </>
              )}
            />
          </Card>
        )}
        {r.byKind.length > 0 && (
          <Card className="overflow-hidden">
            <CardHeader title={t("report.byKind")} />
            <Bars
              groups={r.byKind}
              currency={cur}
              label={(g) => (
                <>
                  <KindIcon icon={g.icon ?? "shapes"} className="size-4 shrink-0 text-fg-subtle" />
                  <span className="truncate">{kindName({ key: g.key ?? "", name: g.name }, t)}</span>
                </>
              )}
            />
          </Card>
        )}
        {r.byTag.length > 0 && (
          <Card className="overflow-hidden">
            <CardHeader title={t("report.byTag")} />
            <Bars
              groups={r.byTag}
              currency={cur}
              link={(g) => `/t/${g.id}`}
              label={(g) => <span className="truncate">{g.name}</span>}
            />
          </Card>
        )}
      </div>

      {r.warrantyEnding.length > 0 && (
        <Section title={t("report.warrantyEnding")}>
          <ThingList things={r.warrantyEnding} />
        </Section>
      )}
    </div>
  );
};

/** Insured things with what they cost, to print or save for the insurer. */
export const InsuranceList = () => {
  const { t } = useTranslation();
  const { data: things = [], isLoading } = useQuery({ queryKey: ["report", "insured"], queryFn: api.insured });
  const { data: household } = useQuery({ queryKey: ["household"], queryFn: api.household });
  const cur = household?.currency ?? "EUR";
  const [today] = useState(() => new Date().toLocaleDateString());
  const totals: Money = {};
  for (const th of things) {
    const c = th.currency || cur;
    if (th.purchasePrice !== null) totals[c] = (totals[c] ?? 0) + th.purchasePrice * th.quantity;
  }
  if (isLoading) return <div className="h-96 animate-pulse rounded-2xl bg-surface-3/60" />;
  return (
    <div className="animate-fade-in flex flex-col gap-5">
      <div className="print:hidden">
        <Breadcrumbs path={[]} current={t("report.insuranceList")} />
      </div>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("report.insuranceTitle", { name: household?.name ?? "" })}
          </h1>
          <p className="mt-1 text-sm text-fg-muted">
            {t("report.insuranceAsOf", { date: today })} · <Amount money={totals} currency={cur} />
          </p>
        </div>
        <button type="button" onClick={() => window.print()} className={linkButton("outline") + " print:hidden"}>
          <Printer /> {t("report.print")}
        </button>
      </div>
      {things.length === 0 ? (
        <Card className="px-6 py-10 text-center text-sm text-fg-muted">{t("report.noInsured")}</Card>
      ) : (
        <div className="overflow-x-auto rounded-2xl border border-border bg-surface print:border-0">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border text-xs text-fg-muted">
              <tr>
                <th className="px-3 py-2 font-medium">{t("thingForm.name")}</th>
                <th className="px-3 py-2 font-medium">{t("report.where")}</th>
                <th className="px-3 py-2 font-medium">{t("thingForm.serial")}</th>
                <th className="px-3 py-2 font-medium">{t("thingForm.purchaseDate")}</th>
                <th className="px-3 py-2 text-right font-medium">{t("thingForm.quantity")}</th>
                <th className="px-3 py-2 text-right font-medium">{t("report.cost")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {things.map((th) => (
                <tr key={th.id} className="break-inside-avoid">
                  <td className="px-3 py-2">
                    <Link to={`/i/${th.id}`} className="font-medium hover:text-primary">
                      {th.name}
                    </Link>
                    {(th.manufacturer || th.model) && (
                      <span className="block text-xs text-fg-subtle">
                        {[th.manufacturer, th.model].filter(Boolean).join(" ")}
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-fg-muted">{th.path.map((p) => p.name).join(" › ")}</td>
                  <td className="px-3 py-2 font-mono text-xs">{th.serial}</td>
                  <td className="px-3 py-2 tabular-nums">{th.purchaseDate}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{th.quantity}</td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {th.purchasePrice !== null ? formatMoney(th.purchasePrice * th.quantity, th.currency || cur) : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
