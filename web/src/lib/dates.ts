import { useTranslation } from "react-i18next";

export const today = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

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

export const addMonths = (iso: string, months: number) => {
  const d = new Date(`${iso}T00:00:00`);
  d.setMonth(d.getMonth() + months);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};
