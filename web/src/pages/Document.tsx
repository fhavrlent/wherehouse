import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Navigate, useParams } from "react-router";
import { toast } from "sonner";
import { Breadcrumbs, ThingList } from "@/components/Things";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/misc";
import { api, errorMessage } from "@/lib/api";

/** What a Paperless document's link back opens: the things it is on. */
export const DocumentPage = () => {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const doc = Number(useParams().doc);
  const { data: things, isLoading } = useQuery({ queryKey: ["document", doc], queryFn: () => api.documentThings(doc) });
  // Other households of the person with things it is on.
  const { data: others, isLoading: loadingOthers } = useQuery({
    queryKey: ["document", doc, "households"],
    queryFn: () => api.documentHouseholds(doc),
  });
  if (isLoading || loadingOthers || !things) return <div className="h-48 animate-pulse rounded-2xl bg-surface-3/60" />;
  if (things.length === 1 && things[0] && !others?.length) return <Navigate to={`/i/${things[0].id}`} replace />;
  const switchTo = async (id: number) => {
    try {
      await api.switchHousehold(id);
      await qc.invalidateQueries();
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };
  return (
    <div className="animate-fade-in flex flex-col gap-5">
      <Breadcrumbs path={[]} current={t("document.title", { id: doc })} />
      <h1 className="text-2xl font-semibold tracking-tight">{t("document.title", { id: doc })}</h1>
      {things.length ? (
        <ThingList things={things} />
      ) : (
        !others?.length && <Card className="px-6 py-10 text-center text-sm text-fg-muted">{t("document.none")}</Card>
      )}
      {others?.map((h) => (
        <Card key={h.id} className="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
          <p className="text-sm text-fg-muted">
            {t(things.length ? "document.otherHousehold" : "document.elsewhere", { name: h.name })}
          </p>
          <Button variant="outline" onClick={() => void switchTo(h.id)}>
            {t("labelPage.switch", { name: h.name })}
          </Button>
        </Card>
      ))}
    </div>
  );
};
