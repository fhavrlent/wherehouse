import { Check } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, errorMessage, type Maintenance, type Repeat } from "@/lib/api";
import { addRepeat, today, useRepeatText } from "@/lib/dates";
import { useInvalidate } from "@/lib/queries";
import { Button } from "./ui/button";
import { Dialog } from "./ui/dialog";
import { Field, Input, Select } from "./ui/input";

const presets: Repeat[] = [
  { every: 1, unit: "week" },
  { every: 2, unit: "week" },
  { every: 1, unit: "month" },
  { every: 2, unit: "month" },
  { every: 3, unit: "month" },
  { every: 6, unit: "month" },
  { every: 12, unit: "month" },
  { every: 24, unit: "month" },
  { every: 36, unit: "month" },
];

const key = (r: Repeat | null) => (r ? `${r.every}-${r.unit}` : "");
const parse = (k: string): Repeat | null => {
  const [every, unit] = k.split("-");
  return every ? { every: Number(every), unit: unit as Repeat["unit"] } : null;
};

/** Picks how often a task comes back, for use inside a Field. */
export const RepeatSelect = ({ value, onChange }: { value: Repeat | null; onChange: (r: Repeat | null) => void }) => {
  const { t } = useTranslation();
  const text = useRepeatText();
  // A repeat set some other way, e.g. through the API, stays a choice.
  const options = value && !presets.some((p) => key(p) === key(value)) ? [...presets, value] : presets;
  return (
    <Select value={key(value)} onChange={(e) => onChange(parse(e.target.value))}>
      <option value="">{t("maintenance.noRepeatOption")}</option>
      {options.map((r) => (
        <option key={key(r)} value={key(r)}>
          {text(r)}
        </option>
      ))}
    </Select>
  );
};

/**
 * Records a due task as done on a day. If it repeats, the next one is added
 * that long after; the repeat can be changed or stopped here.
 */
export const DoneDialog = ({ entry, onClose }: { entry: Maintenance | null; onClose: () => void }) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const [doneOn, setDoneOn] = useState(today);
  const [cost, setCost] = useState("");
  const [repeat, setRepeat] = useState<Repeat | null>(null);
  const [saving, setSaving] = useState(false);
  const [shown, setShown] = useState<Maintenance | null>(null);
  if (entry !== shown) {
    setShown(entry);
    if (entry) {
      setDoneOn(today());
      setCost("");
      setRepeat(entry.repeat);
    }
  }
  const finish = async () => {
    if (!entry) return;
    setSaving(true);
    try {
      const { next } = await api.completeMaintenance(entry.id, {
        doneOn,
        cost: cost.trim() ? Number(cost.replace(",", ".")) : null,
        repeat,
      });
      invalidate();
      if (next?.nextDue) toast.success(t("maintenance.nextAdded", { date: next.nextDue }));
      onClose();
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setSaving(false);
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
          <Button type="submit" form="maintenance-done-form" loading={saving}>
            <Check /> {t("maintenance.markDone")}
          </Button>
        </>
      }
    >
      <form
        id="maintenance-done-form"
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          void finish();
        }}
      >
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("maintenance.doneDate")}>
            <Input type="date" required value={doneOn} onChange={(e) => setDoneOn(e.target.value)} />
          </Field>
          <Field label={t("maintenance.doneCost")} optional>
            <Input
              inputMode="decimal"
              value={cost}
              onChange={(e) => setCost(e.target.value)}
              className="tabular-nums"
            />
          </Field>
        </div>
        <Field
          label={t("maintenance.repeatNext")}
          hint={repeat && doneOn ? t("maintenance.nextAdded", { date: addRepeat(doneOn, repeat) }) : undefined}
        >
          <RepeatSelect value={repeat} onChange={setRepeat} />
        </Field>
      </form>
    </Dialog>
  );
};
