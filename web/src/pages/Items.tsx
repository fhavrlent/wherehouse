import { useTranslation } from "react-i18next";
import { Breadcrumbs, ThingList } from "@/components/Things";
import { Card } from "@/components/ui/misc";
import { useThings } from "@/lib/queries";

/** Things that aren't in anything yet, or archived ones. */
export const ItemsPage = ({ show }: { show: "unplaced" | "archived" }) => {
  const { t } = useTranslation();
  const { data: items, isLoading } = useThings(show === "unplaced" ? { unplaced: true } : { archived: true });
  const title = show === "unplaced" ? t("items.unplacedTitle") : t("items.archivedTitle");
  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex flex-col gap-3">
        <Breadcrumbs path={[]} current={title} />
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        <p className="text-sm text-fg-muted">
          {show === "unplaced" ? t("items.unplacedBody") : t("items.archivedBody")}
        </p>
      </div>
      {isLoading ? (
        <div className="h-48 animate-pulse rounded-2xl bg-surface-3/60" />
      ) : items?.length ? (
        <ThingList things={items} showPath={show === "archived"} />
      ) : (
        <Card className="px-6 py-10 text-center text-sm text-fg-muted">{t("items.none")}</Card>
      )}
    </div>
  );
};
