import { useId, useMemo, type ComponentProps, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { FieldContext, useFieldProps, type FieldIds } from "./field";

/** Ends a Field for what is inside, e.g. a dialog opened from a control in it. */
export const NoField = ({ children }: { children: ReactNode }) => <FieldContext value={null}>{children}</FieldContext>;

export const fieldBase =
  "w-full rounded-lg border border-border bg-surface px-3 text-sm text-fg shadow-card placeholder:text-fg-subtle " +
  "transition-[border-color,box-shadow] outline-none focus:border-primary focus:ring-3 focus:ring-ring/40 " +
  "disabled:opacity-60 aria-invalid:border-bad aria-invalid:focus:ring-bad/25";

export const Input = ({ className, ...props }: ComponentProps<"input">) => (
  <input className={cn(fieldBase, "h-9", className)} {...props} {...useFieldProps(props)} />
);

export const Textarea = ({ className, ...props }: ComponentProps<"textarea">) => (
  <textarea
    className={cn(fieldBase, "min-h-20 py-2 leading-relaxed", className)}
    {...props}
    {...useFieldProps(props)}
  />
);

export const Field = ({
  label,
  hint,
  error,
  children,
  className,
  htmlFor,
  optional,
}: {
  label: ReactNode;
  hint?: ReactNode;
  error?: string | null;
  children: ReactNode;
  className?: string;
  htmlFor?: string;
  optional?: boolean;
}) => {
  const { t } = useTranslation();
  const baseId = useId();
  const id = htmlFor ?? `${baseId}-control`;
  const labelId = `${baseId}-label`;
  const noteId = `${baseId}-note`;
  const noted = !!(error || hint);
  const ids = useMemo<FieldIds>(
    () => ({ id, labelId, describedBy: noted ? noteId : undefined, invalid: !!error }),
    [id, labelId, noteId, noted, error],
  );
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <label id={labelId} htmlFor={id} className="text-[13px] font-medium text-fg">
        {label}
        {/* A space, not a margin, so screen readers don't run the words together. */}
        {optional && <span className="font-normal text-fg-subtle"> {t("common.optional")}</span>}
      </label>
      <FieldContext value={ids}>{children}</FieldContext>
      {error ? (
        <p id={noteId} className="text-xs text-bad">
          <span className="sr-only">{t("common.error")}: </span>
          {error}
        </p>
      ) : hint ? (
        <p id={noteId} className="text-xs text-fg-subtle">
          {hint}
        </p>
      ) : null}
    </div>
  );
};

export const Select = ({ className, children, ...props }: ComponentProps<"select">) => (
  <select
    className={cn(
      fieldBase,
      "h-9 cursor-pointer appearance-none bg-[length:16px] bg-[right_0.6rem_center] bg-no-repeat pr-8",
      "bg-[url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%23888' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E\")]",
      className,
    )}
    {...props}
    {...useFieldProps(props)}
  >
    {children}
  </select>
);
