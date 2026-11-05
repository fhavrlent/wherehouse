import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Tag } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { PlacePicker } from "@/components/PlacePicker";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/misc";
import { api, ApiError, errorMessage } from "@/lib/api";
import { useInvalidate } from "@/lib/queries";

/** Where a label's QR code leads: what it is on, or a choice of what to put it on. */
export const LabelPage = () => {
  const { t } = useTranslation();
  const code = (useParams().code ?? "").toUpperCase();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const invalidate = useInvalidate();
  const [picking, setPicking] = useState(false);
  const { data, error, isLoading } = useQuery({
    queryKey: ["label", code],
    queryFn: () => api.label(code),
    retry: false,
  });

  if (isLoading) return <div className="h-64 animate-pulse rounded-2xl bg-surface-3/60" />;
  if (data?.thing) return <Navigate to={`/i/${data.thing.id}`} replace />;
  const unknown = error instanceof ApiError && error.status === 404;
  if (error && !unknown) return <p className="text-sm text-fg-muted">{errorMessage(error)}</p>;

  // A label of another of the reader's households.
  if (data?.household) {
    const other = data.household;
    return (
      <Card className="mx-auto flex max-w-md flex-col items-center gap-3 px-6 py-10 text-center">
        <p className="font-mono text-2xl font-semibold tracking-[0.2em]">{code}</p>
        <p className="text-sm text-fg-muted">{t("labelPage.otherHousehold", { name: other.name })}</p>
        <Button
          onClick={async () => {
            await api.switchHousehold(other.id);
            await qc.invalidateQueries();
          }}
        >
          {t("labelPage.switch", { name: other.name })}
        </Button>
      </Card>
    );
  }

  const attachTo = async (thingId: number) => {
    try {
      await api.attachLabel(thingId, code);
      invalidate();
      void navigate(`/i/${thingId}`, { replace: true });
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };
  const newThing = async () => {
    try {
      const thing = await api.createThing({
        name: t("labelPage.newName", { code }),
        parentId: null,
        kindId: 0,
        quantity: 1,
        description: "",
        manufacturer: "",
        model: "",
        serial: "",
        notes: "",
      });
      await attachTo(thing.id);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  return (
    <div className="animate-fade-in mx-auto flex max-w-md flex-col gap-5">
      <Card className="flex flex-col items-center gap-3 px-6 py-10 text-center">
        <span className="flex size-12 items-center justify-center rounded-2xl bg-primary-soft text-primary">
          <Tag className="size-6" />
        </span>
        <p className="font-mono text-2xl font-semibold tracking-[0.2em]">{code}</p>
        <p className="text-sm text-fg-muted">{unknown ? t("labelPage.unknown") : t("labelPage.blank")}</p>
        <div className="mt-3 flex flex-wrap justify-center gap-2">
          <Button onClick={() => setPicking(true)}>{t("labelPage.onPlace")}</Button>
          <Button variant="outline" onClick={newThing}>
            {t("labelPage.newThing")}
          </Button>
        </div>
        <p className="mt-2 text-xs text-fg-subtle">{t("labelPage.onThingHint")}</p>
      </Card>
      <PlacePicker
        open={picking}
        onOpenChange={setPicking}
        title={t("labelPage.onPlace")}
        value={null}
        onPick={(id) => id !== null && void attachTo(id)}
      />
    </div>
  );
};
