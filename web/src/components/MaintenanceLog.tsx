import { CalendarClock, Check, Pencil, Plus, Trash2, Wrench } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, errorMessage, type Maintenance, type MaintenanceInput } from "@/lib/api";
import { addMonths, daysUntil, today, useDueText } from "@/lib/dates";
import { useInvalidate } from "@/lib/queries";
import { cn } from "@/lib/utils";
import { Button } from "./ui/button";
import { Dialog } from "./ui/dialog";
import { Field, Input, Textarea } from "./ui/input";
import { Card } from "./ui/misc";

/** A thing's maintenance log: what is due first, then what was done. */
export const MaintenanceLog = ({ thingId, entries }: { thingId: number; entries: Maintenance[] }) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const dueText = useDueText();
  const [editing, setEditing] = useState<Maintenance | "new" | null>(null);
  const [finishing, setFinishing] = useState<Maintenance | null>(null);

  const remove = async (m: Maintenance) => {
    try {
      await api.deleteMaintenance(m.id);
    } catch (e) {
      toast.error(errorMessage(e));
    }
    invalidate();
  };

  return (
    <Card className="overflow-hidden">
      <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-2.5">
        <h2 className="text-sm font-semibold">{t("maintenance.title")}</h2>
        <Button variant="ghost" size="sm" onClick={() => setEditing("new")}>
          <Plus /> {t("maintenance.add")}
        </Button>
      </div>
      {entries.length === 0 ? (
        <p className="px-4 py-4 text-sm text-fg-subtle">{t("maintenance.none")}</p>
      ) : (
        <ul className="divide-y divide-border">
          {entries.map((m) => {
            const left = m.nextDue ? daysUntil(m.nextDue) : null;
            return (
              <li key={m.id} className="group flex items-start gap-3 px-4 py-2.5">
                <span
                  className={cn(
                    "mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg [&_svg]:size-4",
                    left === null
                      ? "bg-surface-3 text-fg-subtle"
                      : left < 0
                        ? "bg-bad-soft text-bad"
                        : left <= 7
                          ? "bg-warn-soft text-warn"
                          : "bg-primary-soft text-primary",
                  )}
                >
                  {left === null ? <Wrench /> : <CalendarClock />}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{m.title}</p>
                  <p className="text-xs text-fg-subtle">
                    {[
                      m.nextDue && `${t("maintenance.due")} ${m.nextDue} (${dueText(m.nextDue)})`,
                      m.doneOn && `${t("maintenance.done")} ${m.doneOn}`,
                      m.cost !== null && t("maintenance.costShort", { cost: m.cost }),
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </p>
                  {m.notes && <p className="mt-1 text-xs whitespace-pre-line text-fg-muted">{m.notes}</p>}
                </div>
                <div className="flex shrink-0 gap-0.5 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100">
                  {m.nextDue && (
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t("maintenance.markDone")}
                      onClick={() => setFinishing(m)}
                    >
                      <Check />
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t("maintenance.edit")}
                    onClick={() => setEditing(m)}
                  >
                    <Pencil />
                  </Button>
                  <Button variant="ghost" size="icon-sm" aria-label={t("maintenance.delete")} onClick={() => remove(m)}>
                    <Trash2 />
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}
      <EntryDialog
        open={editing !== null}
        onOpenChange={(o) => !o && setEditing(null)}
        thingId={thingId}
        entry={editing === "new" ? undefined : (editing ?? undefined)}
      />
      <DoneDialog entry={finishing} onClose={() => setFinishing(null)} />
    </Card>
  );
};

const empty: MaintenanceInput = { title: "", doneOn: null, nextDue: null, notes: "", cost: null };

const EntryDialog = ({
  open,
  onOpenChange,
  thingId,
  entry,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  thingId: number;
  entry?: Maintenance;
}) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const [form, setForm] = useState<MaintenanceInput>(entry ?? empty);
  const [cost, setCost] = useState(entry?.cost?.toString() ?? "");
  const [saving, setSaving] = useState(false);
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setForm(entry ?? { ...empty, doneOn: today() });
      setCost(entry?.cost?.toString() ?? "");
    }
  }
  const save = async () => {
    setSaving(true);
    const input = { ...form, cost: cost.trim() ? Number(cost.replace(",", ".")) : null };
    try {
      if (entry) await api.updateMaintenance(entry.id, input);
      else await api.addMaintenance(thingId, input);
      invalidate();
      onOpenChange(false);
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setSaving(false);
    }
  };
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={entry ? t("maintenance.edit") : t("maintenance.add")}
      size="sm"
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" form="maintenance-form" loading={saving}>
            {t("common.save")}
          </Button>
        </>
      }
    >
      <form
        id="maintenance-form"
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <Field label={t("maintenance.what")}>
          <Input
            data-autofocus
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
            placeholder={t("maintenance.whatPlaceholder")}
          />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("maintenance.doneOn")} optional>
            <Input
              type="date"
              value={form.doneOn ?? ""}
              onChange={(e) => setForm({ ...form, doneOn: e.target.value || null })}
            />
          </Field>
          <Field label={t("maintenance.nextDue")} optional>
            <Input
              type="date"
              value={form.nextDue ?? ""}
              onChange={(e) => setForm({ ...form, nextDue: e.target.value || null })}
            />
          </Field>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {[3, 6, 12, 24].map((months) => (
            <button
              key={months}
              type="button"
              onClick={() => setForm({ ...form, nextDue: addMonths(form.doneOn ?? today(), months) })}
              className="cursor-pointer rounded-md bg-surface-3 px-2 py-1 text-xs text-fg-muted hover:text-fg"
            >
              {t("maintenance.inMonths", { count: months })}
            </button>
          ))}
        </div>
        <Field label={t("maintenance.cost")} optional>
          <Input
            inputMode="decimal"
            value={cost}
            onChange={(e) => setCost(e.target.value)}
            className="w-32 tabular-nums"
          />
        </Field>
        <Field label={t("maintenance.notes")} optional>
          <Textarea value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} />
        </Field>
      </form>
    </Dialog>
  );
};

/** Records a due task as done today, and optionally when it's next due. */
const DoneDialog = ({ entry, onClose }: { entry: Maintenance | null; onClose: () => void }) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const [next, setNext] = useState<number | null>(12);
  const finish = async () => {
    if (!entry) return;
    try {
      const done = today();
      // The done entry stays in the log; a repeat is a new entry.
      await api.updateMaintenance(entry.id, { ...entry, doneOn: done, nextDue: null });
      if (next)
        await api.addMaintenance(entry.thingId, { ...entry, doneOn: null, nextDue: addMonths(done, next), cost: null });
      invalidate();
      onClose();
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };
  return (
    <Dialog
      open={entry !== null}
      onOpenChange={(o) => !o && onClose()}
      title={t("maintenance.doneTitle", { title: entry?.title ?? "" })}
      size="sm"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button onClick={finish}>
            <Check /> {t("maintenance.markDone")}
          </Button>
        </>
      }
    >
      <p className="text-sm text-fg-muted">{t("maintenance.repeat")}</p>
      <div className="mt-3 flex flex-wrap gap-1.5">
        {[null, 1, 3, 6, 12, 24].map((m) => (
          <button
            key={m ?? 0}
            type="button"
            onClick={() => setNext(m)}
            className={cn(
              "cursor-pointer rounded-lg border px-3 py-1.5 text-sm",
              next === m ? "border-primary bg-primary-soft/60 text-primary" : "border-border text-fg-muted",
            )}
          >
            {m === null ? t("maintenance.noRepeat") : t("maintenance.inMonths", { count: m })}
          </button>
        ))}
      </div>
    </Dialog>
  );
};
