import { useQueryClient } from "@tanstack/react-query";
import { Check, ChevronsUpDown, Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { api, errorMessage } from "@/lib/api";
import { useHouseholds } from "@/lib/queries";
import { Button } from "./ui/button";
import { Dialog } from "./ui/dialog";
import { Field, Input } from "./ui/input";
import { Menu, MenuContent, MenuItem, MenuLabel, MenuSeparator, MenuTrigger } from "./ui/menu";

/** Shows which household you're in, switches to another, or adds one. */
export const HouseholdSwitcher = () => {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const { data } = useHouseholds();
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [currency, setCurrency] = useState("EUR");
  if (!data) return null;
  const current = data.households.find((h) => h.id === data.current);

  // Everything shown is of the household, so it all loads again.
  const switchTo = async (id: number) => {
    try {
      await api.switchHousehold(id);
      await qc.invalidateQueries();
      void navigate("/");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };
  const create = async () => {
    try {
      await api.createHousehold(name, currency);
      setAdding(false);
      setName("");
      await qc.invalidateQueries();
      void navigate("/");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  return (
    <>
      <Menu>
        <MenuTrigger asChild>
          <button
            type="button"
            className="flex max-w-40 cursor-pointer items-center gap-1 rounded-lg px-2 py-1 text-sm text-fg-muted hover:bg-surface-3 hover:text-fg"
          >
            <span className="truncate">{current?.name}</span>
            <ChevronsUpDown className="size-3.5 shrink-0" />
          </button>
        </MenuTrigger>
        <MenuContent className="w-60">
          <MenuLabel>{t("households.title")}</MenuLabel>
          {data.households.map((h) => (
            <MenuItem
              key={h.id}
              icon={h.id === data.current ? <Check /> : <span className="size-4" />}
              onSelect={() => h.id !== data.current && void switchTo(h.id)}
            >
              <span className="truncate">{h.name}</span>
              {h.members > 1 && <span className="ml-auto text-xs text-fg-subtle">{h.members}</span>}
            </MenuItem>
          ))}
          <MenuSeparator />
          <MenuItem icon={<Plus />} onSelect={() => setAdding(true)}>
            {t("households.add")}
          </MenuItem>
        </MenuContent>
      </Menu>
      <Dialog
        open={adding}
        onOpenChange={setAdding}
        title={t("households.add")}
        description={t("households.addHint")}
        size="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => setAdding(false)}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" form="household-form" disabled={!name.trim()}>
              {t("households.create")}
            </Button>
          </>
        }
      >
        <form
          id="household-form"
          className="grid gap-4 sm:grid-cols-[1fr_6rem]"
          onSubmit={(e) => {
            e.preventDefault();
            void create();
          }}
        >
          <Field label={t("settings.household.name")}>
            <Input
              data-autofocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("households.namePlaceholder")}
            />
          </Field>
          <Field label={t("settings.household.currency")}>
            <Input
              value={currency}
              maxLength={3}
              className="uppercase"
              onChange={(e) => setCurrency(e.target.value.toUpperCase())}
            />
          </Field>
        </form>
      </Dialog>
    </>
  );
};
