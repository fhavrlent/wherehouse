import { cn } from "@/lib/utils";

export const LogoMark = ({ className }: { className?: string }) => (
  <svg viewBox="0 0 32 32" className={cn("size-7", className)} aria-hidden>
    <rect width="32" height="32" rx="8" className="fill-primary" />
    <path
      d="M8 12.5 16 8l8 4.5v8L16 25l-8-4.5v-8Z"
      fill="none"
      className="stroke-primary-fg"
      strokeWidth="2"
      strokeLinejoin="round"
    />
    <path
      d="M8 12.5 16 17l8-4.5M16 17v8"
      fill="none"
      className="stroke-primary-fg"
      strokeWidth="2"
      strokeLinejoin="round"
    />
  </svg>
);

export const Logo = ({ className }: { className?: string }) => (
  <span className={cn("inline-flex items-center gap-2", className)}>
    <LogoMark />
    <span className="text-[15px] font-semibold tracking-tight">Wherehouse</span>
  </span>
);
