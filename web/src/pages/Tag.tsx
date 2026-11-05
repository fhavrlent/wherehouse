import { Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { TagChip } from "@/components/Tags";
import { Breadcrumbs, ThingList } from "@/components/Things";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Dialog } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { Card } from "@/components/ui/misc";
import { api, errorMessage, TAG_COLORS, type TagColor } from "@/lib/api";
import { useInvalidate, useTag } from "@/lib/queries";
import { cn } from "@/lib/utils";

export const TagPage = () => {
  const { t } = useTranslation();
  const id = Number(useParams().id);
  const navigate = useNavigate();
  const invalidate = useInvalidate();
  const { data: tag, error, isLoading } = useTag(id);
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [name, setName] = useState("");
  const [color, setColor] = useState<TagColor>("slate");

  if (error) return <p className="text-sm text-fg-muted">{errorMessage(error)}</p>;
  if (isLoading || !tag) return <div className="h-64 animate-pulse rounded-2xl bg-surface-3/60" />;

  const save = async () => {
    try {
      await api.updateTag(tag.id, name, color);
      invalidate();
      setEditing(false);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex flex-col gap-3">
        <Breadcrumbs path={[]} current={tag.name} />
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-tight">
            <TagChip tag={tag} link={false} />
            <span className="text-base font-normal text-fg-muted">
              {t("tags.things", { count: tag.things.length })}
            </span>
          </h1>
          <div className="flex gap-2">
            <Button
              variant="outline"
              onClick={() => {
                setName(tag.name);
                setColor(tag.color);
                setEditing(true);
              }}
            >
              <Pencil /> {t("tags.edit")}
            </Button>
            <Button variant="outline" size="icon" aria-label={t("tags.delete")} onClick={() => setDeleting(true)}>
              <Trash2 />
            </Button>
          </div>
        </div>
      </div>
      {tag.things.length ? (
        <ThingList things={tag.things} />
      ) : (
        <Card className="px-6 py-10 text-center text-sm text-fg-muted">{t("items.none")}</Card>
      )}
      <Dialog
        open={editing}
        onOpenChange={setEditing}
        title={t("tags.edit")}
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setEditing(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={save}>{t("common.save")}</Button>
          </>
        }
      >
        <div className="flex flex-col gap-4">
          <Field label={t("tags.name")} hint={t("tags.mergeHint")}>
            <Input data-autofocus value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field label={t("tags.color")}>
            <div className="flex flex-wrap gap-1.5">
              {TAG_COLORS.map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => setColor(c)}
                  className={cn(
                    "cursor-pointer rounded-md",
                    color === c && "ring-2 ring-primary ring-offset-2 ring-offset-surface",
                  )}
                >
                  <TagChip tag={{ id: 0, name: name || tag.name, color: c }} link={false} />
                </button>
              ))}
            </div>
          </Field>
        </div>
      </Dialog>
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={t("tags.deleteTitle", { name: tag.name })}
        description={t("tags.deleteBody")}
        onConfirm={async () => {
          try {
            await api.deleteTag(tag.id);
            invalidate();
            void navigate("/");
          } catch (e) {
            toast.error(errorMessage(e));
          }
        }}
      />
    </div>
  );
};
