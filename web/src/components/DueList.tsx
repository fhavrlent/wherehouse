import { CalendarClock, Check, Repeat as RepeatIcon } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { pathText, type Maintenance } from "@/lib/api";
import { daysUntil, useDueText, useRepeatText } from "@/lib/dates";
import { cn } from "@/lib/utils";
import { Button } from "./ui/button";

/** Maintenance that is due, across things; with onDone, each can be marked done. */
export const DueList = ({ entries, onDone }: { entries: Maintenance[]; onDone?: (m: Maintenance) => void }) => {
  const { t } = useTranslation();
  const dueText = useDueText();
  const repeatText = useRepeatText();
  return (
    <div className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-surface shadow-card">
      {entries.map((m) => {
        const left = m.nextDue ? daysUntil(m.nextDue) : 0;
        return (
          <div key={m.id} className="flex items-center hover:bg-surface-2">
            <Link to={`/i/${m.thingId}`} className="flex min-w-0 flex-1 items-center gap-3 px-3 py-2.5">
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
                <span className="block truncate text-sm font-medium">{m.title}</span>
                <span className="flex items-center gap-1.5 text-xs text-fg-subtle">
                  <span className="truncate" title={m.path?.length ? pathText(m.path) : undefined}>
                    {m.thingName}
                  </span>
                  {m.repeat && (
                    <span className="inline-flex shrink-0 items-center gap-1">
                      <span aria-hidden>·</span>
                      <RepeatIcon className="size-3" aria-hidden />
                      {repeatText(m.repeat)}
                    </span>
                  )}
                </span>
              </span>
              <span className="flex shrink-0 flex-col items-end text-xs">
                <span className={cn("font-medium", left < 0 ? "text-bad" : "text-fg-muted")}>
                  {m.nextDue && dueText(m.nextDue)}
                </span>
                {onDone && <span className="hidden text-fg-subtle tabular-nums sm:block">{m.nextDue}</span>}
              </span>
            </Link>
            {onDone && (
              <Button
                variant="ghost"
                size="icon-sm"
                className="mr-2 shrink-0"
                aria-label={`${t("maintenance.markDone")}: ${m.thingName}, ${m.title}`}
                onClick={() => onDone(m)}
              >
                <Check />
              </Button>
            )}
          </div>
        );
      })}
    </div>
  );
};
