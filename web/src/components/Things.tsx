import { ChevronRight } from "lucide-react";
import { type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { isPlace, pathText, photoUrl, type Ref, type Thing } from "@/lib/api";
import { dragProps, useDropTarget } from "@/lib/drag";
import { cn } from "@/lib/utils";
import { KindIcon } from "./KindIcon";
import { TagChip } from "./Tags";

/** A photo preview, or the kind's icon when there is no photo. */
export const Cover = ({ photo, icon, className }: { photo: number | null; icon: string; className?: string }) => (
  <span
    className={cn(
      "flex shrink-0 items-center justify-center overflow-hidden rounded-xl bg-surface-3 text-fg-subtle [&>span]:size-5 [&>svg]:size-5",
      className,
    )}
  >
    {photo ? (
      <img src={photoUrl(photo, true)} alt="" loading="lazy" className="size-full object-cover" />
    ) : (
      <KindIcon icon={icon} />
    )}
  </span>
);

/** A place, as a card things can be dropped on. */
export const PlaceCard = ({ thing }: { thing: Thing }) => {
  const { t } = useTranslation();
  const drop = useDropTarget(thing.id);
  return (
    <Link
      to={`/i/${thing.id}`}
      {...dragProps([thing.id])}
      {...drop.props}
      className={cn(
        "group flex items-center gap-3 rounded-2xl border border-border bg-surface p-3 shadow-card transition-colors hover:border-border-strong",
        drop.over && "border-primary bg-primary-soft/50 ring-2 ring-primary/30",
      )}
    >
      <Cover photo={thing.cover} icon={thing.kind.icon} className="size-12" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium">{thing.name}</span>
        <span className="block truncate text-xs text-fg-subtle">
          {thing.contains === 0 ? t("place.empty") : t("place.contains", { count: thing.contains })}
        </span>
      </span>
      <ChevronRight className="size-4 text-fg-subtle opacity-0 transition-opacity group-hover:opacity-100" />
    </Link>
  );
};

export const PlaceGrid = ({ things }: { things: Thing[] }) => (
  <div className="grid gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
    {things.map((p) => (
      <PlaceCard key={p.id} thing={p} />
    ))}
  </div>
);

export const ThingRow = ({ thing, showPath = true }: { thing: Thing; showPath?: boolean }) => {
  const { t } = useTranslation();
  return (
    <Link
      to={`/i/${thing.id}`}
      {...dragProps([thing.id])}
      className="flex items-center gap-3 px-3 py-2.5 transition-colors hover:bg-surface-2"
    >
      <Cover photo={thing.cover} icon={thing.kind.icon} className="size-10 rounded-lg" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium">
          {thing.name}
          {thing.soldDate && <span className="ml-1.5 text-xs font-normal text-fg-subtle">· {t("thing.sold")}</span>}
        </span>
        {showPath && (
          <span className="block truncate text-xs text-fg-subtle">
            {thing.path.length ? pathText(thing.path) : t("thing.nowhere")}
          </span>
        )}
      </span>
      {thing.tags.length > 0 && (
        <span className="hidden max-w-48 shrink-0 gap-1 overflow-hidden sm:flex">
          {thing.tags.slice(0, 3).map((tag) => (
            <TagChip key={tag.id} tag={tag} link={false} />
          ))}
        </span>
      )}
      {thing.quantity > 1 && (
        <span className="rounded-md bg-surface-3 px-1.5 py-0.5 text-xs font-medium text-fg-muted tabular-nums">
          ×{thing.quantity}
        </span>
      )}
    </Link>
  );
};

export const ThingList = ({ things, showPath }: { things: Thing[]; showPath?: boolean }) => (
  <div className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-surface shadow-card">
    {things.map((i) => (
      <ThingRow key={i.id} thing={i} showPath={showPath} />
    ))}
  </div>
);

/** Places as cards, then the rest as a list. */
export const Contents = ({ things, showPath }: { things: Thing[]; showPath?: boolean }) => {
  const { t } = useTranslation();
  const places = things.filter(isPlace);
  const rest = things.filter((x) => !isPlace(x));
  return (
    <>
      {places.length > 0 && (
        <Section title={t("thing.places")}>
          <PlaceGrid things={places} />
        </Section>
      )}
      {rest.length > 0 && (
        <Section title={t("thing.things", { count: rest.length })}>
          <ThingList things={rest} showPath={showPath} />
        </Section>
      )}
    </>
  );
};

/** "Things › House › Kitchen", each a link and a place to drop things on. */
export const Breadcrumbs = ({ path, current }: { path: Ref[]; current?: string }) => {
  const { t } = useTranslation();
  return (
    <nav className="flex min-w-0 flex-wrap items-center gap-1 text-sm text-fg-muted">
      <Crumb to="/" into={null}>
        {t("nav.things")}
      </Crumb>
      {path.map((p) => (
        <span key={p.id} className="flex items-center gap-1">
          <ChevronRight className="size-3.5 text-fg-subtle" />
          <Crumb to={`/i/${p.id}`} into={p.id}>
            {p.name}
          </Crumb>
        </span>
      ))}
      {current && (
        <span className="flex min-w-0 items-center gap-1">
          <ChevronRight className="size-3.5 text-fg-subtle" />
          <span className="truncate text-fg">{current}</span>
        </span>
      )}
    </nav>
  );
};

const Crumb = ({ to, into, children }: { to: string; into: number | null; children: ReactNode }) => {
  const drop = useDropTarget(into);
  return (
    <Link
      to={to}
      {...drop.props}
      className={cn(
        "rounded-md px-1 py-0.5 hover:bg-surface-3 hover:text-fg",
        drop.over && "bg-primary-soft text-primary ring-2 ring-primary/30",
      )}
    >
      {children}
    </Link>
  );
};

export const Section = ({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) => (
  <section className="flex flex-col gap-3">
    <div className="flex items-center justify-between gap-3">
      <h2 className="text-sm font-semibold text-fg-muted">{title}</h2>
      {action}
    </div>
    {children}
  </section>
);
