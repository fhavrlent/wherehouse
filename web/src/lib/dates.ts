import { useTranslation } from "react-i18next";
import type { Repeat } from "./api";

const isoDate = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

export const today = () => isoDate(new Date());

/** Days from today to an ISO date; negative when it has passed. */
export const daysUntil = (iso: string) =>
  Math.round((new Date(`${iso}T00:00:00`).getTime() - new Date(`${today()}T00:00:00`).getTime()) / 86_400_000);

/** "in 3 days", "today", "2 days overdue". */
export const useDueText = () => {
  const { t } = useTranslation();
  return (iso: string) => {
    const n = daysUntil(iso);
    if (n === 0) return t("maintenance.dueToday");
    return n > 0 ? t("maintenance.dueIn", { count: n }) : t("maintenance.overdue", { count: -n });
  };
};

/** A month after the 31st is the last day of a shorter month, as on the server. */
export const addMonths = (date: string, months: number) => {
  const d = new Date(`${date}T00:00:00`);
  const day = d.getDate();
  d.setDate(1);
  d.setMonth(d.getMonth() + months);
  d.setDate(Math.min(day, new Date(d.getFullYear(), d.getMonth() + 1, 0).getDate()));
  return isoDate(d);
};

export const addRepeat = (date: string, r: Repeat) => {
  if (r.unit === "month") return addMonths(date, r.every);
  const d = new Date(`${date}T00:00:00`);
  d.setDate(d.getDate() + 7 * r.every);
  return isoDate(d);
};

/** "Every 6 months", "Every year", "Every 2 weeks". */
export const useRepeatText = () => {
  const { t } = useTranslation();
  return (r: Repeat) =>
    r.unit === "week"
      ? t("maintenance.everyWeek", { count: r.every })
      : r.every % 12 === 0
        ? t("maintenance.everyYear", { count: r.every / 12 })
        : t("maintenance.everyMonth", { count: r.every });
};
