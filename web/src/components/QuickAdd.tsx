import { Camera, ImagePlus, Minus, Plus, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { api, errorMessage } from "@/lib/api";
import { useInvalidate, useKinds, useTemplates } from "@/lib/queries";
import { QuickAddContext } from "@/lib/quickAdd";
import { KindSelect } from "./Forms";
import { PlaceField } from "./PlacePicker";
import { Button } from "./ui/button";
import { Dialog } from "./ui/dialog";
import { Field, Input, Select } from "./ui/input";

interface Picked {
  file: File;
  url: string;
}

/** Provides the quick add dialog: a photo and a name, and the thing is added. */
export const QuickAddProvider = ({ children }: { children: ReactNode }) => {
  const [open, setOpen] = useState(false);
  const [placeId, setPlaceId] = useState<number | null>(null);
  const openWith = useCallback((place: number | null = null) => {
    setPlaceId(place);
    setOpen(true);
  }, []);
  return (
    <QuickAddContext value={openWith}>
      {children}
      <QuickAddDialog open={open} onOpenChange={setOpen} placeId={placeId} onPlaceChange={setPlaceId} />
    </QuickAddContext>
  );
};

const QuickAddDialog = ({
  open,
  onOpenChange,
  placeId,
  onPlaceChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  placeId: number | null;
  onPlaceChange: (id: number | null) => void;
}) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const invalidate = useInvalidate();
  const [name, setName] = useState("");
  const [quantity, setQuantity] = useState(1);
  // 0 is the built-in "thing"; it stays as chosen for the next one.
  const [kindId, setKindId] = useState(0);
  const { data: kinds = [] } = useKinds();
  const { data: templates = [] } = useTemplates();
  // A template the thing is made from, with its details, tags and photos.
  const [template, setTemplate] = useState(0);
  const kindValue = kindId || (kinds.find((k) => k.key === "thing")?.id ?? 0);
  const [photos, setPhotos] = useState<Picked[]>([]);
  const [saving, setSaving] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);

  const reset = () => {
    for (const p of photos) URL.revokeObjectURL(p.url);
    setPhotos([]);
    setName("");
    setQuantity(1);
  };
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (!open) reset();
  }
  // Object URLs of photos still shown are freed when the dialog goes away.
  const photosRef = useRef(photos);
  useEffect(() => {
    photosRef.current = photos;
  }, [photos]);
  useEffect(
    () => () => {
      for (const p of photosRef.current) URL.revokeObjectURL(p.url);
    },
    [],
  );

  const addFiles = (files: FileList | null) => {
    if (!files) return;
    const picked = [...files].filter((f) => f.type.startsWith("image/"));
    setPhotos((ps) => [...ps, ...picked.map((file) => ({ file, url: URL.createObjectURL(file) }))]);
    nameRef.current?.focus();
  };

  const save = async (another: boolean) => {
    setSaving(true);
    try {
      const item = template
        ? await api.copyThing(template, { name, parentId: placeId, photos: true })
        : await api.createThing({
            name,
            parentId: placeId,
            kindId,
            quantity,
            description: "",
            manufacturer: "",
            model: "",
            serial: "",
            notes: "",
          });
      if (photos.length) {
        try {
          await api.uploadPhotos(
            item.id,
            photos.map((p) => p.file),
          );
        } catch (e) {
          toast.error(errorMessage(e));
        }
      }
      invalidate();
      toast.success(t("quickAdd.added", { name: item.name }), {
        action: { label: t("quickAdd.open"), onClick: () => void navigate(`/i/${item.id}`) },
      });
      reset();
      if (another) nameRef.current?.focus();
      else onOpenChange(false);
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
      title={t("quickAdd.title")}
      size="sm"
      footer={
        <>
          <Button variant="ghost" onClick={() => save(true)} disabled={saving || !name.trim()}>
            {t("quickAdd.addAnother")}
          </Button>
          <Button type="submit" form="quick-add" loading={saving} disabled={!name.trim()}>
            {t("quickAdd.add")}
          </Button>
        </>
      }
    >
      <form
        id="quick-add"
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          void save(false);
        }}
      >
        {photos.length === 0 ? (
          <div className="grid grid-cols-2 gap-2">
            <PhotoInput capture onFiles={addFiles}>
              <Camera className="size-6" />
              {t("quickAdd.takePhoto")}
            </PhotoInput>
            <PhotoInput onFiles={addFiles}>
              <ImagePlus className="size-6" />
              {t("quickAdd.choosePhoto")}
            </PhotoInput>
          </div>
        ) : (
          <div className="flex gap-2 overflow-x-auto pb-1">
            {photos.map((p) => (
              <div key={p.url} className="relative size-24 shrink-0 overflow-hidden rounded-xl bg-surface-3">
                <img src={p.url} alt="" className="size-full object-cover" />
                <button
                  type="button"
                  aria-label={t("quickAdd.removePhoto")}
                  onClick={() => {
                    URL.revokeObjectURL(p.url);
                    setPhotos((ps) => ps.filter((x) => x !== p));
                  }}
                  className="absolute top-1 right-1 cursor-pointer rounded-full bg-black/55 p-1 text-white hover:bg-black/75"
                >
                  <X className="size-3.5" />
                </button>
              </div>
            ))}
            <PhotoInput onFiles={addFiles} className="size-24">
              <Plus className="size-5" />
            </PhotoInput>
          </div>
        )}
        <Field label={t("quickAdd.name")} htmlFor="quick-name">
          <Input
            ref={nameRef}
            id="quick-name"
            data-autofocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("quickAdd.namePlaceholder")}
            autoComplete="off"
          />
        </Field>
        <div className="grid gap-3 sm:grid-cols-2">
          {templates.length > 0 && (
            <Field label={t("quickAdd.template")}>
              <Select value={template} onChange={(e) => setTemplate(Number(e.target.value))}>
                <option value={0}>{t("quickAdd.noTemplate")}</option>
                {templates.map((tpl) => (
                  <option key={tpl.id} value={tpl.id}>
                    {tpl.name}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          {!template && (
            <Field label={t("quickAdd.kind")}>
              <KindSelect value={kindValue} onChange={setKindId} />
            </Field>
          )}
        </div>
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-3">
          <Field label={t("quickAdd.place")} className="min-w-0">
            <PlaceField
              value={placeId}
              onChange={onPlaceChange}
              noneLabel={t("thing.nowhere")}
              title={t("quickAdd.place")}
            />
          </Field>
          <div className="flex h-9 items-center rounded-lg border border-border bg-surface shadow-card">
            <button
              type="button"
              aria-label={t("quickAdd.less")}
              disabled={quantity <= 1}
              onClick={() => setQuantity((q) => Math.max(1, q - 1))}
              className="flex h-full cursor-pointer items-center px-2 text-fg-muted hover:text-fg disabled:opacity-40"
            >
              <Minus className="size-4" />
            </button>
            <span className="min-w-6 text-center text-sm tabular-nums">{quantity}</span>
            <button
              type="button"
              aria-label={t("quickAdd.more")}
              onClick={() => setQuantity((q) => q + 1)}
              className="flex h-full cursor-pointer items-center px-2 text-fg-muted hover:text-fg"
            >
              <Plus className="size-4" />
            </button>
          </div>
        </div>
      </form>
    </Dialog>
  );
};

/** A button that picks photos; with capture, phones open the camera. */
export const PhotoInput = ({
  capture,
  onFiles,
  children,
  className,
}: {
  capture?: boolean;
  onFiles: (files: FileList | null) => void;
  children: ReactNode;
  className?: string;
}) => (
  <label
    className={
      "flex cursor-pointer flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed border-border-strong " +
      "bg-surface-2 px-3 py-6 text-center text-xs font-medium text-fg-muted transition-colors hover:border-primary hover:text-primary " +
      "focus-within:ring-3 focus-within:ring-ring/40 " +
      (className ?? "")
    }
  >
    {children}
    <input
      type="file"
      accept="image/*"
      multiple={!capture}
      {...(capture ? { capture: "environment" as const } : {})}
      className="sr-only"
      onChange={(e) => {
        onFiles(e.target.files);
        e.target.value = "";
      }}
    />
  </label>
);
