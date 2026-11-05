import { Link2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Field, Select } from "@/components/ui/input";
import { Card } from "@/components/ui/misc";
import { api, errorMessage } from "@/lib/api";
import { useHouseholds } from "@/lib/queries";

/** Where another app sends people to let it use Wherehouse as them. */
export const Connect = () => {
  const { t } = useTranslation();
  const [params] = useSearchParams();
  const app = params.get("app") ?? "";
  const redirect = params.get("redirect_uri") ?? "";
  const state = params.get("state") ?? "";
  const { data } = useHouseholds();
  const [household, setHousehold] = useState(0);
  const [busy, setBusy] = useState(false);
  let host = "";
  try {
    host = new URL(redirect).host;
  } catch {
    /* shown as invalid below */
  }
  if (!app || !host) {
    return (
      <Card className="mx-auto max-w-md px-6 py-10 text-center text-sm text-fg-muted">{t("connect.invalid")}</Card>
    );
  }
  const chosen = household || (data?.current ?? 0);
  const allow = async () => {
    setBusy(true);
    try {
      const { redirect: to } = await api.allowConnect(app, redirect, state, chosen);
      location.assign(to);
    } catch (e) {
      toast.error(errorMessage(e));
      setBusy(false);
    }
  };
  const deny = () => {
    const u = new URL(redirect);
    u.searchParams.set("error", "access_denied");
    u.searchParams.set("state", state);
    location.assign(u.toString());
  };
  return (
    <Card className="mx-auto flex max-w-md flex-col gap-4 px-6 py-8">
      <div className="flex flex-col items-center gap-3 text-center">
        <span className="flex size-12 items-center justify-center rounded-2xl bg-primary-soft text-primary">
          <Link2 className="size-6" />
        </span>
        <h1 className="text-lg font-semibold">{t("connect.title", { app })}</h1>
        <p className="text-sm text-fg-muted">{t("connect.body", { app, host })}</p>
      </div>
      {data && data.households.length > 1 && (
        <Field label={t("connect.household")}>
          <Select value={chosen} onChange={(e) => setHousehold(Number(e.target.value))}>
            {data.households.map((h) => (
              <option key={h.id} value={h.id}>
                {h.name}
              </option>
            ))}
          </Select>
        </Field>
      )}
      <p className="text-xs text-fg-subtle">{t("connect.hint")}</p>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={deny}>
          {t("common.cancel")}
        </Button>
        <Button onClick={allow} loading={busy} disabled={!chosen}>
          {t("connect.allow")}
        </Button>
      </div>
    </Card>
  );
};
