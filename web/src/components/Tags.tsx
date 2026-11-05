import { X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import type { Tag, TagColor } from "@/lib/api";
import { useTags } from "@/lib/queries";
import { cn } from "@/lib/utils";
import { fieldBase } from "./ui/input";

// Written out so Tailwind sees every class.
const colors: Record<TagColor, string> = {
  slate: "bg-slate-500/15 text-slate-700 dark:text-slate-300",
  red: "bg-red-500/15 text-red-700 dark:text-red-300",
  orange: "bg-orange-500/15 text-orange-700 dark:text-orange-300",
  amber: "bg-amber-500/20 text-amber-800 dark:text-amber-300",
  lime: "bg-lime-500/20 text-lime-800 dark:text-lime-300",
  green: "bg-green-500/15 text-green-700 dark:text-green-300",
  teal: "bg-teal-500/15 text-teal-700 dark:text-teal-300",
  sky: "bg-sky-500/15 text-sky-700 dark:text-sky-300",
  blue: "bg-blue-500/15 text-blue-700 dark:text-blue-300",
  violet: "bg-violet-500/15 text-violet-700 dark:text-violet-300",
  pink: "bg-pink-500/15 text-pink-700 dark:text-pink-300",
};

export const TagChip = ({ tag, link = true }: { tag: Pick<Tag, "id" | "name" | "color">; link?: boolean }) => {
  const cls = cn("inline-flex h-6 items-center rounded-md px-2 text-xs font-medium", colors[tag.color]);
  return link ? (
    <Link to={`/t/${tag.id}`} className={cn(cls, "hover:opacity-80")}>
      {tag.name}
    </Link>
  ) : (
    <span className={cls}>{tag.name}</span>
  );
};

/** Edits a list of tag names, suggesting tags that exist. */
export const TagInput = ({ value, onChange }: { value: string[]; onChange: (tags: string[]) => void }) => {
  const { t } = useTranslation();
  const { data: all = [] } = useTags();
  const [text, setText] = useState("");
  const add = (name: string) => {
    const n = name.trim().replace(/^#/, "");
    if (n && !value.some((v) => v.toLowerCase() === n.toLowerCase())) onChange([...value, n]);
    setText("");
  };
  const q = text.trim().toLowerCase();
  const suggestions = q
    ? all.filter(
        (tag) => tag.name.toLowerCase().includes(q) && !value.some((v) => v.toLowerCase() === tag.name.toLowerCase()),
      )
    : [];
  const colorOf = (name: string) => all.find((tag) => tag.name.toLowerCase() === name.toLowerCase())?.color ?? "slate";

  return (
    <div className="relative">
      <div className={cn(fieldBase, "flex min-h-9 flex-wrap items-center gap-1.5 py-1.5")}>
        {value.map((name) => (
          <span
            key={name}
            className={cn(
              "inline-flex h-6 items-center gap-1 rounded-md pr-1 pl-2 text-xs font-medium",
              colors[colorOf(name)],
            )}
          >
            {name}
            <button
              type="button"
              aria-label={t("tags.remove", { name })}
              onClick={() => onChange(value.filter((v) => v !== name))}
              className="cursor-pointer rounded p-0.5 hover:bg-black/10"
            >
              <X className="size-3" />
            </button>
          </span>
        ))}
        <input
          value={text}
          onChange={(e) => {
            const v = e.target.value;
            if (v.endsWith(",")) add(v.slice(0, -1));
            else setText(v);
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" && text.trim()) {
              e.preventDefault();
              add(text);
            } else if (e.key === "Backspace" && !text && value.length) {
              onChange(value.slice(0, -1));
            }
          }}
          onBlur={() => text.trim() && add(text)}
          placeholder={value.length ? "" : t("tags.placeholder")}
          className="min-w-24 flex-1 bg-transparent text-sm outline-none placeholder:text-fg-subtle"
        />
      </div>
      {suggestions.length > 0 && (
        <div className="absolute inset-x-0 top-full z-10 mt-1 flex flex-wrap gap-1.5 rounded-lg border border-border bg-surface p-2 shadow-pop">
          {suggestions.slice(0, 12).map((tag) => (
            <button
              key={tag.id}
              type="button"
              onMouseDown={(e) => {
                e.preventDefault();
                add(tag.name);
              }}
              className="cursor-pointer"
            >
              <TagChip tag={tag} link={false} />
            </button>
          ))}
        </div>
      )}
    </div>
  );
};
