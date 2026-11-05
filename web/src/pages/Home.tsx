import { Archive, Boxes, Inbox, Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { ThingFormDialog } from "@/components/Forms";
import { SearchBox } from "@/components/SearchBox";
import { DueList } from "@/components/DueList";
import { TagChip } from "@/components/Tags";
import { PlaceGrid, Section, ThingList } from "@/components/Things";
import { Button } from "@/components/ui/button";
import { linkButton } from "@/components/ui/buttonStyles";
import { Card } from "@/components/ui/misc";
import { useDropTarget } from "@/lib/drag";
import { useDue, useHome, useTags } from "@/lib/queries";
import { useQuickAdd } from "@/lib/quickAdd";
import { cn } from "@/lib/utils";

export const Home = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const quickAdd = useQuickAdd();
  const { data, isLoading } = useHome();
  const { data: due = [] } = useDue();
  const { data: tags = [] } = useTags();
  const [adding, setAdding] = useState(false);
  // Dropping onto the page's heading moves a place to the top level.
  const top = useDropTarget(null);

  if (isLoading || !data) return <div className="h-96 animate-pulse rounded-2xl bg-surface-3/60" />;
  const empty = data.places.length === 0 && data.recent.length === 0;

  return (
    <div className="animate-fade-in flex flex-col gap-8">
      <SearchBox className="md:hidden" />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1
          {...top.props}
          className={cn("rounded-lg text-2xl font-semibold tracking-tight", top.over && "bg-primary-soft text-primary")}
        >
          {t("home.title")}
        </h1>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setAdding(true)}>
            <Plus /> {t("home.addPlace")}
          </Button>
        </div>
      </div>

      {empty ? (
        <Card className="flex flex-col items-center px-6 py-16 text-center">
          <span className="flex size-12 items-center justify-center rounded-2xl bg-primary-soft text-primary">
            <Boxes className="size-6" />
          </span>
          <h2 className="mt-4 text-base font-semibold">{t("home.emptyTitle")}</h2>
          <p className="mt-1.5 max-w-sm text-sm text-fg-muted">{t("home.emptyBody")}</p>
          <div className="mt-6 flex flex-wrap justify-center gap-2">
            <Button onClick={() => setAdding(true)}>
              <Plus /> {t("home.addFirstPlace")}
            </Button>
            <Button variant="outline" onClick={() => quickAdd()}>
              {t("home.addThing")}
            </Button>
          </div>
        </Card>
      ) : (
        <>
          {due.length > 0 && (
            <Section title={t("home.due")}>
              <DueList entries={due} />
            </Section>
          )}
          {data.places.length > 0 && (
            <Section title={t("home.places")}>
              <PlaceGrid things={data.places} />
            </Section>
          )}
          {data.recent.length > 0 && (
            <Section title={t("home.recent")}>
              <ThingList things={data.recent} />
            </Section>
          )}
          {tags.length > 0 && (
            <Section title={t("home.tags")}>
              <div className="flex flex-wrap gap-1.5">
                {tags.map((tag) => (
                  <TagChip key={tag.id} tag={tag} />
                ))}
              </div>
            </Section>
          )}
          <div className="flex flex-wrap gap-2">
            {data.loose > 0 && (
              <Link to="/unplaced" className={linkButton("outline")}>
                <Inbox /> {t("home.unplaced", { count: data.loose })}
              </Link>
            )}
            <Link to="/archived" className={linkButton("ghost")}>
              <Archive /> {t("home.archived")}
            </Link>
          </div>
        </>
      )}

      <ThingFormDialog
        open={adding}
        onOpenChange={setAdding}
        kindKey="building"
        onSaved={(p) => void navigate(`/i/${p.id}`)}
      />
    </div>
  );
};
