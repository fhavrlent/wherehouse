import { Search, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate, useSearchParams } from "react-router";
import { cn } from "@/lib/utils";
import { Kbd } from "./ui/misc";

/** Searches as you type, showing results on the search page. "/" focuses it. */
export const SearchBox = ({ className }: { className?: string }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [params] = useSearchParams();
  const onSearch = pathname === "/search";
  const [value, setValue] = useState(onSearch ? (params.get("q") ?? "") : "");
  const ref = useRef<HTMLInputElement>(null);

  // Leaving the search page clears the box.
  const [wasOnSearch, setWasOnSearch] = useState(onSearch);
  if (onSearch !== wasOnSearch) {
    setWasOnSearch(onSearch);
    if (!onSearch) setValue("");
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (e.key !== "/" || target.closest("input, textarea, select, [contenteditable]")) return;
      e.preventDefault();
      ref.current?.focus();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const change = (q: string) => {
    setValue(q);
    const to = `/search?q=${encodeURIComponent(q)}`;
    // Typing replaces the history entry, so Back leaves the search page.
    void navigate(to, { replace: onSearch });
  };

  return (
    <label className={cn("relative block", className)}>
      <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-fg-subtle" />
      <input
        ref={ref}
        type="search"
        value={value}
        onChange={(e) => change(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            setValue("");
            ref.current?.blur();
            if (onSearch) void navigate(-1);
          }
        }}
        placeholder={t("search.placeholder")}
        aria-label={t("search.label")}
        className="h-9 w-full rounded-lg border border-border bg-surface-2 pr-9 pl-9 text-sm outline-none placeholder:text-fg-subtle focus:border-primary focus:bg-surface focus:ring-3 focus:ring-ring/40"
      />
      {value ? (
        <button
          type="button"
          onClick={() => {
            change("");
            ref.current?.focus();
          }}
          className="absolute top-1/2 right-1.5 -translate-y-1/2 cursor-pointer rounded-md p-1 text-fg-subtle hover:text-fg"
          aria-label={t("search.clear")}
        >
          <X className="size-4" />
        </button>
      ) : (
        <span className="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2">
          <Kbd>/</Kbd>
        </span>
      )}
    </label>
  );
};
