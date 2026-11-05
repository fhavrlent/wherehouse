import { ImagePlus, Loader2, Star, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, errorMessage, photoUrl, type Photo } from "@/lib/api";
import { useInvalidate } from "@/lib/queries";
import { PhotoInput } from "./QuickAdd";
import { Button } from "./ui/button";
import { Dialog } from "./ui/dialog";

/** The photos of a thing, with adding, choosing the cover and deleting. */
export const Photos = ({ id, photos }: { id: number; photos: Photo[] }) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const [uploading, setUploading] = useState(false);
  const [shown, setShown] = useState<Photo | null>(null);

  const add = async (files: FileList | null) => {
    if (!files?.length) return;
    setUploading(true);
    try {
      await api.uploadPhotos(id, [...files]);
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setUploading(false);
      invalidate();
    }
  };

  const act = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
      setShown(null);
    } catch (e) {
      toast.error(errorMessage(e));
    }
    invalidate();
  };

  return (
    <div className="flex flex-wrap gap-2">
      {photos.map((p, i) => (
        <button
          key={p.id}
          type="button"
          onClick={() => setShown(p)}
          className={
            "relative cursor-zoom-in overflow-hidden rounded-xl bg-surface-3 " +
            (i === 0 ? "size-40" : "size-24 sm:size-28")
          }
        >
          <img src={photoUrl(p.id, true)} alt="" loading="lazy" className="size-full object-cover" />
        </button>
      ))}
      <PhotoInput onFiles={add} className={photos.length ? "size-24 sm:size-28" : "h-28 w-full sm:w-56"}>
        {uploading ? <Loader2 className="size-5 animate-spin" /> : <ImagePlus className="size-5" />}
        {!photos.length && t("photos.add")}
      </PhotoInput>
      <Dialog
        open={shown !== null}
        onOpenChange={(o) => !o && setShown(null)}
        title={t("photos.title")}
        size="xl"
        footer={
          shown && (
            <>
              <Button variant="ghost" className="mr-auto text-bad" onClick={() => act(() => api.deletePhoto(shown.id))}>
                <Trash2 /> {t("photos.delete")}
              </Button>
              {photos[0]?.id !== shown.id && (
                <Button variant="outline" onClick={() => act(() => api.makeCover(shown.id))}>
                  <Star /> {t("photos.makeCover")}
                </Button>
              )}
              <a
                href={photoUrl(shown.id)}
                target="_blank"
                rel="noreferrer"
                className="inline-flex h-9 items-center rounded-lg border border-border bg-surface px-3.5 text-sm font-medium shadow-card hover:bg-surface-2"
              >
                {t("photos.original")}
              </a>
            </>
          )
        }
      >
        {shown && <img src={photoUrl(shown.id)} alt="" className="mx-auto max-h-[70dvh] rounded-lg object-contain" />}
      </Dialog>
    </div>
  );
};
