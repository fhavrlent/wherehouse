import { Link2 } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Field, Select } from "@/components/ui/input";
import { Card } from "@/components/ui/misc";
import { api, errorMessage } from "@/lib/api";
import { useHouseholds } from "@/lib/queries";
import { cn } from "@/lib/utils";

/** Where another app sends people to let it use Wherehouse as them. */
export const Connect = () => {
  const { t } = useTranslation();
  const [params] = useSearchParams();
  const app = params.get("app") ?? "";
  const redirect = params.get("redirect_uri") ?? "";
  const state = params.get("state") ?? "";
  // PKCE: only the app that started this can swap the code for a token.
  const challenge = params.get("code_challenge") ?? "";
  const method = params.get("code_challenge_method") ?? "";
  // multiple=1: the person ticks households, and the app gets a token for each.
  const multiple = params.get("multiple") === "1";
  const preselect = Number(params.get("household")) || 0;
  const { data } = useHouseholds();
  const [household, setHousehold] = useState(0);
  const [ticked, setTicked] = useState<number[] | null>(null);
  const [busy, setBusy] = useState(false);
  const many = useQuery({
    queryKey: ["connect", app, redirect],
    queryFn: () => api.connectHouseholds(app, redirect),
    enabled: multiple && !!app && !!redirect,
  });
  let host = "";
  try {
    host = new URL(redirect).host;
  } catch {
    /* shown as invalid below */
  }
  if (!app || !host || !challenge || method !== "S256") {
    return (
      <Card className="mx-auto max-w-md px-6 py-10 text-center text-sm text-fg-muted">{t("connect.invalid")}</Card>
    );
  }
  const chosen = household || preselect || (data?.current ?? 0);
  const open = (many.data ?? []).filter((h) => !h.connected);
  // Until the person ticks something: the household asked for, else the
  // current one, if the app isn't connected to it yet.
  const picked =
    ticked ??
    open.filter((h) => h.id === (open.some((o) => o.id === preselect) ? preselect : data?.current)).map((h) => h.id);
  const toggle = (id: number) => setTicked(picked.includes(id) ? picked.filter((x) => x !== id) : [...picked, id]);
  const allow = async () => {
    setBusy(true);
    try {
      const { redirect: to } = multiple
        ? await api.allowConnectMany(app, redirect, state, challenge, picked)
        : await api.allowConnect(app, redirect, state, challenge, chosen);
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
        <p className="text-sm text-fg-muted">{t(multiple ? "connect.bodyMany" : "connect.body", { app, host })}</p>
      </div>
      {multiple && many.data && (
        <fieldset className="flex flex-col gap-1.5">
          <legend className="mb-1.5 text-sm font-medium">{t("connect.households")}</legend>
          <div className="divide-y divide-border rounded-xl border border-border">
            {many.data.map((h) => (
              <label
                key={h.id}
                className={cn(
                  "flex items-center gap-3 px-3 py-2.5 text-sm",
                  h.connected ? "text-fg-muted" : "cursor-pointer hover:bg-surface-2",
                )}
              >
                <input
                  type="checkbox"
                  className="size-4 accent-primary"
                  checked={h.connected || picked.includes(h.id)}
                  disabled={h.connected}
                  onChange={() => toggle(h.id)}
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{h.name}</span>
                  <span className="block text-xs text-fg-subtle">{t("connect.members", { count: h.members })}</span>
                </span>
                {h.connected && <span className="text-xs">{t("connect.connected")}</span>}
              </label>
            ))}
          </div>
          {open.length === 0 && <p className="text-xs text-fg-subtle">{t("connect.allConnected", { app })}</p>}
        </fieldset>
      )}
      {!multiple && data && data.households.length > 1 && (
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
        <Button onClick={allow} loading={busy} disabled={multiple ? picked.length === 0 : !chosen}>
          {t("connect.allow")}
        </Button>
      </div>
    </Card>
  );
};
