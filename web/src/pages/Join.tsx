import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Home } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/misc";
import { api, errorMessage } from "@/lib/api";

/** Where an invite link leads: join the household. */
export const Join = () => {
  const { t } = useTranslation();
  const token = useParams().token ?? "";
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { data, error, isLoading } = useQuery({
    queryKey: ["invite", token],
    queryFn: () => api.inviteInfo(token),
    retry: false,
  });
  if (isLoading) return <div className="h-64 animate-pulse rounded-2xl bg-surface-3/60" />;
  return (
    <Card className="mx-auto flex max-w-md flex-col items-center gap-3 px-6 py-10 text-center">
      <span className="flex size-12 items-center justify-center rounded-2xl bg-primary-soft text-primary">
        <Home className="size-6" />
      </span>
      {error || !data ? (
        <p className="text-sm text-fg-muted">{errorMessage(error)}</p>
      ) : (
        <>
          <h1 className="text-lg font-semibold">{t("join.title", { name: data.name })}</h1>
          <p className="text-sm text-fg-muted">
            {data.member ? t("join.already") : t("join.body", { count: data.members })}
          </p>
          <Button
            className="mt-2"
            onClick={async () => {
              try {
                await api.acceptInvite(token);
                await qc.invalidateQueries();
                void navigate("/", { replace: true });
              } catch (e) {
                toast.error(errorMessage(e));
              }
            }}
          >
            {data.member ? t("join.open") : t("join.join")}
          </Button>
        </>
      )}
    </Card>
  );
};
