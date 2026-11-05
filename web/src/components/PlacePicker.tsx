import { Check, Plus, Search } from "lucide-react";
import { useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, errorMessage, pathText, type Thing } from "@/lib/api";
import { useInvalidate, useKinds, usePlaces } from "@/lib/queries";
import { cn } from "@/lib/utils";
import { KindIcon } from "./KindIcon";
import { Dialog } from "./ui/dialog";
import { Input } from "./ui/input";

/** Picks a place from all of them, by typing part of its name or path. */
export const PlacePicker = ({
  open,
  onOpenChange,
  title,
  value,
  exclude,
  noneLabel,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  value: number | null;
  /** A thing that, with everything inside it, can't be picked. */
  exclude?: number;
  /** The choice of no place, e.g. "Top level"; omitted, there is none. */
  noneLabel?: string;
  onPick: (id: number | null, place?: Thing) => void;
}) => {
  const { t } = useTranslation();
  const { data: places = [] } = usePlaces();
  const { data: kinds = [] } = useKinds();
  const invalidate = useInvalidate();
  const [query, setQuery] = useState("");
  const [creating, setCreating] = useState(false);

  const choices = useMemo(() => {
    const q = query.trim().toLowerCase();
    return places.filter(
      (p) =>
        p.id !== exclude &&
        !p.path.some((r) => r.id === exclude) &&
        (!q || `${pathText(p.path)} ${p.name}`.toLowerCase().includes(q)),
    );
  }, [places, query, exclude]);

  const pick = (id: number | null, place?: Thing) => {
    onPick(id, place);
    onOpenChange(false);
    setQuery("");
  };

  const name = query.trim();
  const create = async () => {
    setCreating(true);
    try {
      const room = kinds.find((k) => k.key === "room");
      const p = await api.createThing({
        name,
        parentId: null,
        kindId: room?.id ?? 0,
        description: "",
        quantity: 1,
        manufacturer: "",
        model: "",
        serial: "",
        notes: "",
      });
      invalidate();
      pick(p.id, p);
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setCreating(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={title} size="sm" className="sm:top-[15%] sm:translate-y-0">
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-fg-subtle" />
        <Input
          data-autofocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("picker.search")}
          className="pl-9"
          onKeyDown={(e) => {
            if (e.key === "Enter" && choices[0]) {
              e.preventDefault();
              pick(choices[0].id, choices[0]);
            }
          }}
        />
      </div>
      <div className="-mx-2 mt-3 flex max-h-80 flex-col overflow-y-auto">
        {noneLabel && !query && <Choice label={noneLabel} selected={value === null} onClick={() => pick(null)} />}
        {choices.map((p) => (
          <Choice
            key={p.id}
            label={p.name}
            detail={pathText(p.path)}
            icon={<KindIcon icon={p.kind.icon} className="size-4" />}
            selected={value === p.id}
            depth={query ? 0 : p.path.length}
            onClick={() => pick(p.id, p)}
          />
        ))}
        {name && !places.some((p) => p.name.toLowerCase() === name.toLowerCase()) && (
          <button
            type="button"
            disabled={creating}
            onClick={create}
            className="flex cursor-pointer items-center gap-2.5 rounded-lg px-2 py-2 text-left text-sm text-primary hover:bg-surface-2"
          >
            <Plus className="size-4" />
            {t("picker.create", { name })}
          </button>
        )}
        {!name && choices.length === 0 && <p className="px-2 py-3 text-sm text-fg-subtle">{t("picker.noPlaces")}</p>}
      </div>
    </Dialog>
  );
};

const Choice = ({
  label,
  detail,
  icon,
  selected,
  depth = 0,
  onClick,
}: {
  label: string;
  detail?: string;
  icon?: ReactNode;
  selected: boolean;
  depth?: number;
  onClick: () => void;
}) => (
  <button
    type="button"
    onClick={onClick}
    style={{ paddingLeft: `${0.5 + depth * 1.1}rem` }}
    className={cn(
      "flex cursor-pointer items-center gap-2.5 rounded-lg py-2 pr-2 text-left text-sm hover:bg-surface-2",
      selected && "bg-primary-soft/60",
    )}
  >
    <span className="text-fg-subtle">{icon}</span>
    <span className="min-w-0 flex-1">
      <span className="block truncate">{label}</span>
      {detail && depth === 0 && <span className="block truncate text-xs text-fg-subtle">{detail}</span>}
    </span>
    {selected && <Check className="size-4 text-primary" />}
  </button>
);

/** A button showing the chosen place, which opens the picker. */
export const PlaceField = ({
  value,
  onChange,
  noneLabel,
  exclude,
  title,
}: {
  value: number | null;
  onChange: (id: number | null) => void;
  noneLabel: string;
  exclude?: number;
  title: string;
}) => {
  const { data: places = [] } = usePlaces();
  const [open, setOpen] = useState(false);
  const place = places.find((p) => p.id === value);
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="flex h-9 w-full cursor-pointer items-center gap-2 rounded-lg border border-border bg-surface px-3 text-left text-sm shadow-card hover:border-border-strong"
      >
        {place ? (
          <>
            <KindIcon icon={place.kind.icon} className="size-4 shrink-0 text-fg-subtle" />
            <span className="min-w-0 truncate">
              {place.path.length > 0 && <span className="text-fg-subtle">{pathText(place.path)} › </span>}
              {place.name}
            </span>
          </>
        ) : (
          <span className="text-fg-muted">{noneLabel}</span>
        )}
      </button>
      <PlacePicker
        open={open}
        onOpenChange={setOpen}
        title={title}
        value={value}
        exclude={exclude}
        noneLabel={noneLabel}
        onPick={onChange}
      />
    </>
  );
};
