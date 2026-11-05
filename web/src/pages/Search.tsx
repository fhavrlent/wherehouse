import { SearchX } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { SearchBox } from "@/components/SearchBox";
import { Contents } from "@/components/Things";
import { Card, Segmented } from "@/components/ui/misc";
import { useSearch } from "@/lib/queries";

export const SearchPage = () => {
  const { t } = useTranslation();
  const [params] = useSearchParams();
  const q = params.get("q") ?? "";
  const [archived, setArchived] = useState(false);
  const { data, isFetching } = useSearch(q, archived);
  const none = data?.length === 0;

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SearchBox className="md:hidden" />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold tracking-tight">
          {q.trim() ? t("search.title", { q: q.trim() }) : t("search.empty")}
        </h1>
        <Segmented
          size="sm"
          value={archived ? "archived" : "current"}
          onChange={(v) => setArchived(v === "archived")}
          options={[
            { value: "current", label: t("search.current") },
            { value: "archived", label: t("search.archived") },
          ]}
        />
      </div>
      {q.trim() && none && !isFetching && (
        <Card className="flex flex-col items-center px-6 py-12 text-center">
          <SearchX className="size-6 text-fg-subtle" />
          <p className="mt-3 text-sm text-fg-muted">{t("search.noResults")}</p>
        </Card>
      )}
      {data && q.trim() && <Contents things={data} />}
    </div>
  );
};
