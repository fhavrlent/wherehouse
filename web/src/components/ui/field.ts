import { createContext, use, type ComponentProps } from "react";

/** What a control inside a Field is labelled and described by. */
export interface FieldIds {
  id: string;
  labelId: string;
  describedBy: string | undefined;
  invalid: boolean;
}

export const FieldContext = createContext<FieldIds | null>(null);

/** The ids of the Field a control is in, or null outside one. */
export const useField = () => use(FieldContext);

/**
 * Props tying a control to its Field: the label, the hint or error under it,
 * and whether it is invalid. A control with its own id or aria-label is a
 * second control in the Field (such as the currency beside a price) and
 * keeps only what it was given.
 */
export const useFieldProps = (props: {
  id?: string;
  "aria-label"?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: ComponentProps<"input">["aria-invalid"];
}) => {
  const field = useField();
  if (!field || props.id !== undefined || props["aria-label"] !== undefined) return {};
  const describedBy = [props["aria-describedby"], field.describedBy].filter(Boolean).join(" ");
  return {
    id: field.id,
    "aria-describedby": describedBy || undefined,
    "aria-invalid": props["aria-invalid"] ?? (field.invalid || undefined),
  };
};
