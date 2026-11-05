import { CalendarClock } from "lucide-react";
import { Link } from "react-router";
import { pathText, type Maintenance } from "@/lib/api";
import { daysUntil, useDueText } from "@/lib/dates";
import { cn } from "@/lib/utils";

/** Maintenance that is due, across things. */
export const DueList = ({ entries }: { entries: Maintenance[] }) => {
  const dueText = useDueText();
  return (
    <div className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-surface shadow-card">
      {entries.map((m) => {
        const left = m.nextDue ? daysUntil(m.nextDue) : 0;
        return (
          <Link key={m.id} to={`/i/${m.thingId}`} className="flex items-center gap-3 px-3 py-2.5 hover:bg-surface-2">
            <span
              className={cn(
                "flex size-10 shrink-0 items-center justify-center rounded-lg",
                left < 0
                  ? "bg-bad-soft text-bad"
                  : left <= 7
                    ? "bg-warn-soft text-warn"
                    : "bg-primary-soft text-primary",
              )}
            >
              <CalendarClock className="size-5" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">
                {m.thingName}: {m.title}
              </span>
              <span className="block truncate text-xs text-fg-subtle">{m.path?.length ? pathText(m.path) : ""}</span>
            </span>
            <span className={cn("shrink-0 text-xs font-medium", left < 0 ? "text-bad" : "text-fg-muted")}>
              {m.nextDue && dueText(m.nextDue)}
            </span>
          </Link>
        );
      })}
    </div>
  );
};
