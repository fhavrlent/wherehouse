import { ExternalLink, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, errorMessage, type Link } from "@/lib/api";
import { appName } from "@/lib/apps";
import { useInvalidate } from "@/lib/queries";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Card, CardHeader } from "./ui/misc";

/** Links from a thing elsewhere: its warranty, the shop's page, a video. */
export const Links = ({ thingId, links }: { thingId: number; links: Link[] }) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const [adding, setAdding] = useState(false);
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const run = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
      return true;
    } catch (e) {
      toast.error(errorMessage(e));
      return false;
    } finally {
      invalidate();
    }
  };
  return (
    <Card className="overflow-hidden">
      <CardHeader title={t("links.title")}>
        <Button variant="ghost" size="sm" onClick={() => setAdding(true)}>
          <Plus /> {t("links.add")}
        </Button>
      </CardHeader>
      {links.length === 0 && !adding && <p className="px-4 py-4 text-sm text-fg-subtle">{t("links.none")}</p>}
      {links.length > 0 && (
        <ul className="divide-y divide-border">
          {links.map((l) => (
            <li key={l.id} className="group flex items-center gap-3 px-4 py-2">
              <ExternalLink className="size-4 shrink-0 text-fg-subtle" />
              <a
                href={l.url}
                target="_blank"
                rel="noreferrer"
                className="min-w-0 flex-1 truncate text-sm hover:text-primary"
              >
                {l.title || l.url.replace(/^https?:\/\//, "")}
              </a>
              {l.source && (
                <span className="rounded bg-primary-soft px-1.5 py-0.5 text-[11px] text-primary">
                  {appName(l.source)}
                </span>
              )}
              <button
                type="button"
                aria-label={t("links.delete")}
                onClick={() => run(() => api.deleteLink(l.id))}
                className="cursor-pointer rounded-md p-1 text-fg-subtle opacity-100 hover:bg-surface-3 hover:text-bad sm:opacity-0 sm:group-hover:opacity-100"
              >
                <Trash2 className="size-4" />
              </button>
            </li>
          ))}
        </ul>
      )}
      {adding && (
        <form
          className="flex flex-wrap gap-2 border-t border-border bg-surface-2 px-4 py-3"
          onSubmit={(e) => {
            e.preventDefault();
            void run(() => api.addLink(thingId, url, title)).then((ok) => {
              if (ok) {
                setUrl("");
                setTitle("");
                setAdding(false);
              }
            });
          }}
        >
          <Input
            autoFocus
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://"
            className="min-w-48 flex-[2]"
          />
          <Input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder={t("links.titlePlaceholder")}
            className="min-w-32 flex-1"
          />
          <Button type="submit" variant="outline" disabled={!url.trim()}>
            {t("links.add")}
          </Button>
        </form>
      )}
    </Card>
  );
};
