import { Printer, Trash2, X } from "lucide-react";
import { useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { QrCode } from "@/components/QrCode";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/input";
import { Card } from "@/components/ui/misc";
import { api, errorMessage } from "@/lib/api";
import {
  labelUrl,
  printList,
  saveSheet,
  savedSheet,
  SHEETS,
  usePrintList,
  type QueuedLabel,
  type Sheet,
} from "@/lib/labels";

/** 1 mm in CSS pixels, for sizing the preview. */
const MM = 96 / 25.4;

export const Labels = () => {
  const { t } = useTranslation();
  const list = usePrintList();
  const [sheet, setSheet] = useState<Sheet>(savedSheet);
  const [skip, setSkip] = useState("0");
  const [blank, setBlank] = useState("10");
  const [making, setMaking] = useState(false);

  const perPage = sheet.columns * sheet.rows;
  const skipped = Math.min(Math.max(parseInt(skip, 10) || 0, 0), perPage - 1);
  const slots: (QueuedLabel | null)[] = [...Array<null>(skipped).fill(null), ...list];
  const pages: (QueuedLabel | null)[][] = [];
  for (let i = 0; i < slots.length; i += perPage) pages.push(slots.slice(i, i + perPage));

  const makeBlank = async () => {
    setMaking(true);
    try {
      const { codes } = await api.blankLabels(parseInt(blank, 10) || 0);
      printList.add(codes.map((code) => ({ code, name: "", detail: "" })));
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setMaking(false);
    }
  };

  // Small previews: A4 sheets about 400px wide, rolls a little bigger.
  const zoom = Math.min(400 / (sheet.page[0] * MM), 1.6);

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{t("labels.title")}</h1>
          <p className="mt-1 text-sm text-fg-muted">{t("labels.description")}</p>
        </div>
        <Button onClick={() => window.print()} disabled={list.length === 0}>
          <Printer /> {t("labels.print", { count: list.length })}
        </Button>
      </div>

      <div className="grid gap-6 lg:grid-cols-[1fr_auto]">
        <div className="flex flex-col gap-4">
          <Card className="grid gap-4 p-5 sm:grid-cols-[1fr_9rem]">
            <Field label={t("labels.sheet")}>
              <Select
                value={sheet.id}
                onChange={(e) => {
                  const s = SHEETS.find((x) => x.id === e.target.value);
                  if (s) {
                    setSheet(s);
                    saveSheet(s.id);
                  }
                }}
              >
                {SHEETS.map((s) => (
                  <option key={s.id} value={s.id}>
                    {t(`sheets.${s.id}` as "sheets.a4-21")}
                  </option>
                ))}
              </Select>
            </Field>
            {perPage > 1 && (
              <Field label={t("labels.skip")} hint={t("labels.skipHint")}>
                <Input
                  inputMode="numeric"
                  value={skip}
                  onChange={(e) => setSkip(e.target.value.replace(/\D/g, ""))}
                  className="tabular-nums"
                />
              </Field>
            )}
          </Card>

          <Card className="overflow-hidden">
            <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
              <h2 className="text-sm font-semibold">{t("labels.list", { count: list.length })}</h2>
              {list.length > 0 && (
                <Button variant="ghost" size="sm" onClick={printList.clear}>
                  <Trash2 /> {t("labels.clear")}
                </Button>
              )}
            </div>
            {list.length === 0 ? (
              <p className="px-4 py-6 text-sm text-fg-muted">{t("labels.empty")}</p>
            ) : (
              <ul className="max-h-80 divide-y divide-border overflow-y-auto">
                {list.map((l) => (
                  <li key={l.code} className="flex items-center gap-3 px-4 py-2">
                    <span className="w-16 font-mono text-xs text-fg-muted">{l.code}</span>
                    <span className="min-w-0 flex-1 truncate text-sm">
                      {l.name || <span className="text-fg-subtle">{t("labels.blank")}</span>}
                    </span>
                    <button
                      type="button"
                      aria-label={t("labels.removeOne")}
                      onClick={() => printList.remove(l.code)}
                      className="cursor-pointer rounded-md p-1 text-fg-subtle hover:bg-surface-3 hover:text-fg"
                    >
                      <X className="size-4" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <form
              className="flex flex-wrap items-end gap-2 border-t border-border bg-surface-2 px-4 py-3"
              onSubmit={(e) => {
                e.preventDefault();
                void makeBlank();
              }}
            >
              <Field label={t("labels.blankCount")} hint={t("labels.blankHint")} className="min-w-0 flex-1">
                <div className="flex gap-2">
                  <Input
                    inputMode="numeric"
                    value={blank}
                    onChange={(e) => setBlank(e.target.value.replace(/\D/g, ""))}
                    className="w-24 tabular-nums"
                  />
                  <Button type="submit" variant="outline" loading={making}>
                    {t("labels.addBlank")}
                  </Button>
                </div>
              </Field>
            </form>
          </Card>
        </div>

        {pages.length > 0 && (
          <div className="flex flex-col gap-3">
            <h2 className="text-sm font-semibold text-fg-muted">{t("labels.preview", { count: pages.length })}</h2>
            <div className="w-fit overflow-hidden rounded-md shadow-pop" style={{ zoom }}>
              <Page sheet={sheet} labels={pages[0] ?? []} />
            </div>
          </div>
        )}
      </div>

      {createPortal(
        <div className="print-only">
          <style>{`@page { size: ${sheet.page[0]}mm ${sheet.page[1]}mm; margin: 0; }`}</style>
          {pages.map((p, i) => (
            // eslint-disable-next-line @eslint-react/no-array-index-key -- pages have no identity of their own
            <Page key={i} sheet={sheet} labels={p} />
          ))}
        </div>,
        document.body,
      )}
    </div>
  );
};

const Page = ({ sheet, labels }: { sheet: Sheet; labels: (QueuedLabel | null)[] }) => (
  <div
    className="label-page relative overflow-hidden bg-white text-black"
    style={{ width: `${sheet.page[0]}mm`, height: `${sheet.page[1]}mm` }}
  >
    {labels.map((l, i) => {
      const col = i % sheet.columns;
      const row = Math.floor(i / sheet.columns);
      return (
        l && (
          <div
            key={l.code}
            className="absolute"
            style={{
              left: `${sheet.margin[1] + col * (sheet.label[0] + sheet.gap[1])}mm`,
              top: `${sheet.margin[0] + row * (sheet.label[1] + sheet.gap[0])}mm`,
              width: `${sheet.label[0]}mm`,
              height: `${sheet.label[1]}mm`,
            }}
          >
            <LabelFace label={l} size={sheet.label} />
          </div>
        )
      );
    })}
  </div>
);

const LabelFace = ({ label, size: [w, h] }: { label: QueuedLabel; size: [number, number] }) => {
  const pad = Math.min(2, h * 0.08);
  const qr = h - 2 * pad;
  // Text sizes follow the label's height, within what stays readable.
  const name = Math.min(3.6, Math.max(2, h * 0.13));
  const code = Math.min(4.2, Math.max(2.4, h * 0.14));
  return (
    <div className="flex size-full items-center" style={{ padding: `${pad}mm`, gap: `${pad}mm` }}>
      <div className="shrink-0" style={{ width: `${qr}mm`, height: `${qr}mm` }}>
        <QrCode text={labelUrl(label.code)} className="size-full" />
      </div>
      <div className="flex min-w-0 flex-1 flex-col justify-center" style={{ maxWidth: `${w - qr - 3 * pad}mm` }}>
        {label.name && (
          <span className="line-clamp-2 leading-tight font-semibold" style={{ fontSize: `${name}mm` }}>
            {label.name}
          </span>
        )}
        {label.detail && h >= 25 && (
          <span className="mt-[0.5mm] truncate leading-tight text-neutral-600" style={{ fontSize: `${name * 0.75}mm` }}>
            {label.detail}
          </span>
        )}
        <span className="mt-[0.8mm] font-mono font-semibold tracking-wider" style={{ fontSize: `${code}mm` }}>
          {label.code}
        </span>
      </div>
    </div>
  );
};
