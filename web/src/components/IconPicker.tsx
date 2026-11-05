import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Search, Trash2, Upload } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, errorMessage } from "@/lib/api";
import { ICON_NAMES } from "@/lib/icons";
import { keys } from "@/lib/queries";
import { cn } from "@/lib/utils";
import { KindIcon } from "./KindIcon";
import { Dialog } from "./ui/dialog";
import { Input } from "./ui/input";

/** Picks a built-in icon or one of the household's own, and uploads new ones. */
export const IconPicker = ({
  open,
  onOpenChange,
  value,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  value: string;
  onPick: (icon: string) => void;
}) => {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [q, setQ] = useState("");
  const [uploading, setUploading] = useState(false);
  const { data: custom = [] } = useQuery({ queryKey: keys.icons, queryFn: api.icons, enabled: open });
  const pick = (icon: string) => {
    onPick(icon);
    onOpenChange(false);
  };
  const refresh = () => qc.invalidateQueries({ queryKey: keys.icons });
  const upload = async (files: FileList | null) => {
    const f = files?.[0];
    if (!f) return;
    setUploading(true);
    try {
      const icon = await api.uploadIcon(f);
      await refresh();
      pick(`custom:${icon.id}`);
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setUploading(false);
    }
  };
  const names = ICON_NAMES.filter((n) => n.includes(q.trim().toLowerCase()));
  const cell = (selected: boolean) =>
    cn(
      "flex size-10 cursor-pointer items-center justify-center rounded-lg text-fg-muted hover:bg-surface-3 hover:text-fg [&>*]:size-5",
      selected && "bg-primary-soft text-primary ring-1 ring-primary/40",
    );

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t("icons.title")} size="md">
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-fg-subtle" />
          <Input
            data-autofocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={t("icons.search")}
            className="pl-9"
          />
        </div>
        <label className="inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-lg border border-border bg-surface px-3 text-sm font-medium shadow-card hover:bg-surface-2">
          {uploading ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
          {t("icons.upload")}
          <input
            type="file"
            accept="image/svg+xml,image/png,image/webp,image/jpeg"
            className="sr-only"
            onChange={(e) => {
              void upload(e.target.files);
              e.target.value = "";
            }}
          />
        </label>
      </div>
      {custom.length > 0 && !q && (
        <>
          <h3 className="mt-4 mb-2 text-xs font-semibold text-fg-muted">{t("icons.yours")}</h3>
          <div className="flex flex-wrap gap-1">
            {custom.map((c) => {
              const id = `custom:${c.id}`;
              return (
                <div key={c.id} className="group relative">
                  <button type="button" onClick={() => pick(id)} className={cell(value === id)}>
                    <KindIcon icon={id} />
                  </button>
                  <button
                    type="button"
                    aria-label={t("icons.delete")}
                    onClick={async () => {
                      try {
                        await api.deleteIcon(c.id);
                        await refresh();
                      } catch (e) {
                        toast.error(errorMessage(e));
                      }
                    }}
                    className="absolute -top-1 -right-1 hidden cursor-pointer rounded-full bg-surface p-0.5 text-fg-subtle shadow-card group-hover:block hover:text-bad"
                  >
                    <Trash2 className="size-3" />
                  </button>
                </div>
              );
            })}
          </div>
          <p className="mt-1 text-xs text-fg-subtle">{t("icons.uploadHint")}</p>
        </>
      )}
      <h3 className="mt-4 mb-2 text-xs font-semibold text-fg-muted">{t("icons.builtIn")}</h3>
      <div className="grid max-h-72 grid-cols-[repeat(auto-fill,minmax(2.5rem,1fr))] gap-1 overflow-y-auto">
        {names.map((n) => (
          <button key={n} type="button" title={n} onClick={() => pick(n)} className={cell(value === n)}>
            <KindIcon icon={n} />
          </button>
        ))}
      </div>
    </Dialog>
  );
};
