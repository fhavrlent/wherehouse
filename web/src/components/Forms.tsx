import { ChevronDown } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, errorMessage, toInput, type Field as CustomField, type Thing, type ThingInput } from "@/lib/api";
import { appName } from "@/lib/apps";
import { kindName } from "@/lib/kinds";
import { useFields, useHousehold, useInvalidate, useKinds } from "@/lib/queries";
import { cn } from "@/lib/utils";
import { KindIcon } from "./KindIcon";
import { PlaceField } from "./PlacePicker";
import { TagInput } from "./Tags";
import { Button } from "./ui/button";
import { Dialog } from "./ui/dialog";
import { Field, Input, Select, Textarea } from "./ui/input";
import { Switch } from "./ui/misc";

type Form = ThingInput & { tags: string[]; fields: Record<number, string> };

const blank = (parentId: number | null, kindId: number): Form => ({
  parentId,
  kindId,
  name: "",
  description: "",
  quantity: 1,
  manufacturer: "",
  model: "",
  serial: "",
  notes: "",
  purchaseDate: null,
  purchasePrice: null,
  purchaseFrom: "",
  currency: "",
  warrantyUntil: null,
  lifetimeWarranty: false,
  warrantyDetails: "",
  insured: false,
  soldDate: null,
  soldPrice: null,
  soldTo: "",
  soldNotes: "",
  tags: [],
  fields: {},
});

/** A kind picker, places first. */
export const KindSelect = ({ value, onChange }: { value: number; onChange: (id: number) => void }) => {
  const { t } = useTranslation();
  const { data: kinds = [] } = useKinds();
  const kind = kinds.find((k) => k.id === value);
  return (
    <div className="relative">
      {kind && (
        <KindIcon
          icon={kind.icon}
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-fg-subtle"
        />
      )}
      <Select value={value} onChange={(e) => onChange(Number(e.target.value))} className="pl-9">
        {kinds.map((k) => (
          <option key={k.id} value={k.id}>
            {kindName(k, t)}
          </option>
        ))}
      </Select>
    </div>
  );
};

/** An input for a custom field's value, by the field's type. */
const FieldInput = ({
  field,
  value,
  onChange,
}: {
  field: CustomField;
  value: string;
  onChange: (v: string) => void;
}) => {
  switch (field.type) {
    case "bool":
      return <Switch checked={value === "true"} onCheckedChange={(on) => onChange(on ? "true" : "false")} />;
    case "date":
      return <Input type="date" value={value} onChange={(e) => onChange(e.target.value)} className="w-44" />;
    case "number":
      return (
        <Input inputMode="decimal" value={value} onChange={(e) => onChange(e.target.value)} className="tabular-nums" />
      );
    case "url":
      return <Input type="url" value={value} onChange={(e) => onChange(e.target.value)} placeholder="https://" />;
    default:
      return <Input value={value} onChange={(e) => onChange(e.target.value)} />;
  }
};

/** A part of the form that is open when there's something in it, or on a click. */
const Group = ({ title, open, children }: { title: string; open: boolean; children: ReactNode }) => {
  const [shown, setShown] = useState(open);
  return (
    <div className="rounded-xl border border-border">
      <button
        type="button"
        onClick={() => setShown(!shown)}
        className="flex w-full cursor-pointer items-center justify-between px-3 py-2.5 text-sm font-medium"
      >
        {title}
        <ChevronDown className={cn("size-4 text-fg-subtle transition-transform", shown && "rotate-180")} />
      </button>
      {shown && <div className="flex flex-col gap-4 border-t border-border px-3 py-4">{children}</div>}
    </div>
  );
};

const price = (v: string) => (v.trim() ? Number(v.replace(/\s/g, "").replace(",", ".")) : null);

/** Adds a thing (without thing) inside parentId, or edits thing. */
export const ThingFormDialog = ({
  open,
  onOpenChange,
  thing,
  parentId = null,
  kindKey = "thing",
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  thing?: Thing;
  parentId?: number | null;
  /** The built-in kind a new thing starts as. */
  kindKey?: string;
  onSaved?: (t: Thing) => void;
}) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const { data: customFields = [] } = useFields();
  const { data: kinds = [] } = useKinds();
  const { data: household } = useHousehold();
  const start = () => (thing ? toInput(thing) : blank(parentId, kinds.find((k) => k.key === kindKey)?.id ?? 0));
  const [form, setForm] = useState<Form>(start);
  const [quantity, setQuantity] = useState(String(form.quantity));
  const [prices, setPrices] = useState({ purchase: "", sold: "" });
  const [saving, setSaving] = useState(false);
  const [wasOpen, setWasOpen] = useState(false);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      const f = start();
      setForm(f);
      setQuantity(String(f.quantity));
      setPrices({ purchase: f.purchasePrice?.toString() ?? "", sold: f.soldPrice?.toString() ?? "" });
    }
  }
  const set = (patch: Partial<Form>) => setForm({ ...form, ...patch });
  const kind = kinds.find((k) => k.id === form.kindId);

  const save = async () => {
    setSaving(true);
    const input: ThingInput = {
      ...form,
      quantity: parseInt(quantity, 10) || 0,
      purchasePrice: price(prices.purchase),
      soldPrice: price(prices.sold),
    };
    try {
      const saved = thing ? await api.updateThing(thing.id, input) : await api.createThing(input);
      invalidate();
      onOpenChange(false);
      onSaved?.(saved);
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setSaving(false);
    }
  };

  const hasPurchase =
    !!(form.purchaseDate || prices.purchase || form.purchaseFrom || form.warrantyUntil) ||
    !!form.lifetimeWarranty ||
    !!form.insured;

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={thing ? t("thingForm.editTitle") : kind?.isPlace ? t("thingForm.addPlace") : t("thingForm.addTitle")}
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" form="thing-form" loading={saving}>
            {thing ? t("common.save") : t("thingForm.add")}
          </Button>
        </>
      }
    >
      <form
        id="thing-form"
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <div className="grid gap-4 sm:grid-cols-[1fr_7rem]">
          <Field label={t("thingForm.name")} htmlFor="thing-name">
            <Input
              id="thing-name"
              data-autofocus
              value={form.name}
              onChange={(e) => set({ name: e.target.value })}
              placeholder={kind?.isPlace ? t("thingForm.placePlaceholder") : ""}
            />
          </Field>
          <Field label={t("thingForm.quantity")} htmlFor="thing-quantity">
            <Input
              id="thing-quantity"
              inputMode="numeric"
              value={quantity}
              onChange={(e) => setQuantity(e.target.value.replace(/\D/g, ""))}
              className="tabular-nums"
            />
          </Field>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("thingForm.kind")}>
            <KindSelect value={form.kindId} onChange={(kindId) => set({ kindId })} />
          </Field>
          <Field label={t("thingForm.in")}>
            <PlaceField
              value={form.parentId}
              onChange={(parentId) => set({ parentId })}
              noneLabel={t("thing.nowhere")}
              exclude={thing?.id}
              title={t("thingForm.in")}
            />
          </Field>
        </div>
        <div className="grid gap-4 sm:grid-cols-3">
          <Field label={t("thingForm.manufacturer")} optional>
            <Input value={form.manufacturer} onChange={(e) => set({ manufacturer: e.target.value })} />
          </Field>
          <Field label={t("thingForm.model")} optional>
            <Input value={form.model} onChange={(e) => set({ model: e.target.value })} />
          </Field>
          <Field label={t("thingForm.serial")} optional>
            <Input value={form.serial} onChange={(e) => set({ serial: e.target.value })} />
          </Field>
        </div>
        <Field label={t("thingForm.tags")} optional>
          <TagInput value={form.tags} onChange={(tags) => set({ tags })} />
        </Field>

        <Group title={t("thingForm.purchase")} open={hasPurchase}>
          {thing?.purchaseOwner && (
            <p className="rounded-lg bg-surface-2 px-3 py-2 text-xs text-fg-muted">
              {t("thingForm.ownedBy", { app: appName(thing.purchaseOwner) })}
            </p>
          )}
          <fieldset disabled={!!thing?.purchaseOwner} className="contents">
            <div className="grid gap-4 sm:grid-cols-3">
              <Field label={t("thingForm.purchaseDate")}>
                <Input
                  type="date"
                  value={form.purchaseDate ?? ""}
                  onChange={(e) => set({ purchaseDate: e.target.value || null })}
                />
              </Field>
              <Field label={t("thingForm.price")} hint={t("thingForm.priceHint")}>
                <div className="flex gap-2">
                  <Input
                    inputMode="decimal"
                    value={prices.purchase}
                    onChange={(e) => setPrices({ ...prices, purchase: e.target.value })}
                    className="tabular-nums"
                  />
                  <Input
                    value={form.currency ?? ""}
                    onChange={(e) => set({ currency: e.target.value.toUpperCase().slice(0, 3) })}
                    placeholder={household?.currency ?? "EUR"}
                    className="w-16 uppercase"
                    aria-label={t("thingForm.currency")}
                  />
                </div>
              </Field>
              <Field label={t("thingForm.purchaseFrom")}>
                <Input value={form.purchaseFrom ?? ""} onChange={(e) => set({ purchaseFrom: e.target.value })} />
              </Field>
            </div>
            <div className="grid items-end gap-4 sm:grid-cols-2">
              <Field label={t("thingForm.warrantyUntil")}>
                <Input
                  type="date"
                  value={form.warrantyUntil ?? ""}
                  disabled={form.lifetimeWarranty}
                  onChange={(e) => set({ warrantyUntil: e.target.value || null })}
                />
              </Field>
              <label className="flex h-9 items-center gap-2 text-sm">
                <Switch
                  checked={form.lifetimeWarranty ?? false}
                  onCheckedChange={(lifetimeWarranty) => set({ lifetimeWarranty })}
                />
                {t("thingForm.lifetime")}
              </label>
            </div>
            <Field label={t("thingForm.warrantyDetails")} optional>
              <Input value={form.warrantyDetails ?? ""} onChange={(e) => set({ warrantyDetails: e.target.value })} />
            </Field>
          </fieldset>
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={form.insured ?? false} onCheckedChange={(insured) => set({ insured })} />
            {t("thingForm.insured")}
          </label>
        </Group>

        <Group title={t("thingForm.sold")} open={!!form.soldDate}>
          <div className="grid gap-4 sm:grid-cols-3">
            <Field label={t("thingForm.soldDate")}>
              <Input
                type="date"
                value={form.soldDate ?? ""}
                onChange={(e) => set({ soldDate: e.target.value || null })}
              />
            </Field>
            <Field label={t("thingForm.soldPrice")}>
              <Input
                inputMode="decimal"
                value={prices.sold}
                onChange={(e) => setPrices({ ...prices, sold: e.target.value })}
                className="tabular-nums"
              />
            </Field>
            <Field label={t("thingForm.soldTo")}>
              <Input value={form.soldTo ?? ""} onChange={(e) => set({ soldTo: e.target.value })} />
            </Field>
          </div>
          <Field label={t("thingForm.soldNotes")} optional>
            <Input value={form.soldNotes ?? ""} onChange={(e) => set({ soldNotes: e.target.value })} />
          </Field>
        </Group>

        {customFields.length > 0 && (
          <div className="grid gap-4 sm:grid-cols-2">
            {customFields.map((f) => (
              <Field key={f.id} label={f.name} optional>
                <FieldInput
                  field={f}
                  value={form.fields[f.id] ?? ""}
                  onChange={(v) => set({ fields: { ...form.fields, [f.id]: v } })}
                />
              </Field>
            ))}
          </div>
        )}
        <Field label={t("thingForm.description")} optional>
          <Textarea value={form.description} onChange={(e) => set({ description: e.target.value })} />
        </Field>
        <Field label={t("thingForm.notes")} optional>
          <Textarea value={form.notes} onChange={(e) => set({ notes: e.target.value })} />
        </Field>
      </form>
    </Dialog>
  );
};
