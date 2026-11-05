import type { ComponentProps } from "react";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { sizes, variants, type Size, type Variant } from "./buttonStyles";

export type ButtonProps = ComponentProps<"button"> & {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
};

export const Button = ({
  className,
  variant = "primary",
  size = "md",
  loading = false,
  disabled = false,
  children,
  type = "button",
  ...props
}: ButtonProps) => (
  <button
    type={type}
    disabled={disabled || loading}
    className={cn(
      "inline-flex shrink-0 cursor-pointer items-center justify-center font-medium whitespace-nowrap transition-colors select-none",
      "disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0",
      variants[variant],
      sizes[size],
      className,
    )}
    {...props}
  >
    {loading && <Loader2 className="animate-spin" />}
    {children}
  </button>
);
