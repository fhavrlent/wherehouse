import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Navigate, useParams } from "react-router";
import { Breadcrumbs, ThingList } from "@/components/Things";
import { Card } from "@/components/ui/misc";
import { api } from "@/lib/api";

/** What a Paperless document's link back opens: the things it is on. */
export const DocumentPage = () => {
  const { t } = useTranslation();
  const doc = Number(useParams().doc);
  const { data: things, isLoading } = useQuery({ queryKey: ["document", doc], queryFn: () => api.documentThings(doc) });
  if (isLoading || !things) return <div className="h-48 animate-pulse rounded-2xl bg-surface-3/60" />;
  if (things.length === 1 && things[0]) return <Navigate to={`/i/${things[0].id}`} replace />;
  return (
    <div className="animate-fade-in flex flex-col gap-5">
      <Breadcrumbs path={[]} current={t("document.title", { id: doc })} />
      <h1 className="text-2xl font-semibold tracking-tight">{t("document.title", { id: doc })}</h1>
      {things.length ? (
        <ThingList things={things} />
      ) : (
        <Card className="px-6 py-10 text-center text-sm text-fg-muted">{t("document.none")}</Card>
      )}
    </div>
  );
};
