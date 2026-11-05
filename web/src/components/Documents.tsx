import { useQuery } from "@tanstack/react-query";
import { ChevronDown, Download, FileText, Library, Loader2, Search, Trash2, Upload } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, ATTACHMENT_KINDS, attachmentUrl, errorMessage, type Attachment, type AttachmentKind } from "@/lib/api";
import { useInvalidate, useSettingsQuery } from "@/lib/queries";
import { Button } from "./ui/button";
import { Dialog } from "./ui/dialog";
import { Input, Select } from "./ui/input";
import { Card, CardHeader } from "./ui/misc";

/** A thing's documents: manuals, receipts and so on, stored here or linked from Paperless. */
export const Documents = ({ thingId, documents }: { thingId: number; documents: Attachment[] }) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const { data: settings } = useSettingsQuery();
  const [uploading, setUploading] = useState(false);
  const [linking, setLinking] = useState(false);

  const upload = async (files: FileList | null) => {
    if (!files?.length) return;
    setUploading(true);
    try {
      for (const f of files) {
        const kind: AttachmentKind = /manual|návod|anleitung/i.test(f.name) ? "manual" : "other";
        await api.uploadAttachment(thingId, f, kind);
      }
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setUploading(false);
      invalidate();
    }
  };
  const run = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
    } catch (e) {
      toast.error(errorMessage(e));
    }
    invalidate();
  };

  return (
    <Card className="overflow-hidden">
      <CardHeader title={t("documents.title")}>
        {settings?.paperlessEnabled && (
          <Button variant="ghost" size="sm" onClick={() => setLinking(true)}>
            <Library /> {t("documents.fromPaperless")}
          </Button>
        )}
        <label className="inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-lg px-3 text-[13px] font-medium text-fg-muted hover:bg-surface-3 hover:text-fg">
          {uploading ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
          {t("documents.upload")}
          <input
            type="file"
            multiple
            className="sr-only"
            onChange={(e) => {
              void upload(e.target.files);
              e.target.value = "";
            }}
          />
        </label>
      </CardHeader>
      {documents.length === 0 ? (
        <p className="px-4 py-4 text-sm text-fg-subtle">{t("documents.none")}</p>
      ) : (
        <ul className="divide-y divide-border">
          {documents.map((d) => (
            <li key={d.id} className="flex items-start gap-3 py-2.5 pr-2 pl-4">
              <FileText className="mt-0.5 size-4 shrink-0 text-fg-subtle" />
              <div className="min-w-0 flex-1">
                <a
                  href={attachmentUrl(d)}
                  target="_blank"
                  rel="noreferrer"
                  className="line-clamp-2 text-sm break-words hover:text-primary"
                >
                  {d.filename}
                </a>
                <div className="mt-0.5 flex items-center gap-1.5 text-xs text-fg-subtle">
                  <label className="relative -ml-1 inline-flex items-center rounded text-fg-muted focus-within:ring-2 focus-within:ring-ring hover:bg-surface-3 hover:text-fg">
                    <select
                      value={d.kind}
                      onChange={(e) =>
                        run(() => api.updateAttachment(d.id, e.target.value as AttachmentKind, d.filename))
                      }
                      className="[field-sizing:content] cursor-pointer appearance-none bg-transparent py-0.5 pr-4 pl-1 text-xs outline-none"
                      aria-label={t("documents.kind")}
                    >
                      {ATTACHMENT_KINDS.map((k) => (
                        <option key={k} value={k}>
                          {t(`documentKinds.${k}`)}
                        </option>
                      ))}
                    </select>
                    <ChevronDown className="pointer-events-none absolute right-0.5 size-3" />
                  </label>
                  {d.storage === "paperless" && <span>Paperless</span>}
                </div>
              </div>
              <a
                href={attachmentUrl(d, true)}
                aria-label={t("documents.download")}
                className="rounded-md p-1 text-fg-subtle hover:bg-surface-3 hover:text-fg"
              >
                <Download className="size-4" />
              </a>
              <button
                type="button"
                aria-label={t("documents.delete")}
                onClick={() => run(() => api.deleteAttachment(d.id))}
                className="cursor-pointer rounded-md p-1 text-fg-subtle hover:bg-surface-3 hover:text-bad"
              >
                <Trash2 className="size-4" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <PaperlessPicker open={linking} onOpenChange={setLinking} thingId={thingId} />
    </Card>
  );
};

const PaperlessPicker = ({
  open,
  onOpenChange,
  thingId,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  thingId: number;
}) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const [q, setQ] = useState("");
  const [kind, setKind] = useState<AttachmentKind>("manual");
  const docs = useQuery({
    queryKey: ["paperless", q],
    queryFn: () => api.searchPaperless(q),
    enabled: open && q.trim().length > 1,
  });
  const link = async (id: number) => {
    try {
      await api.linkPaperless(thingId, [id], kind);
      invalidate();
      onOpenChange(false);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t("documents.fromPaperless")} size="md">
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-fg-subtle" />
          <Input
            data-autofocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={t("documents.searchPaperless")}
            className="pl-9"
          />
        </div>
        <Select value={kind} onChange={(e) => setKind(e.target.value as AttachmentKind)} className="w-36">
          {ATTACHMENT_KINDS.map((k) => (
            <option key={k} value={k}>
              {t(`documentKinds.${k}`)}
            </option>
          ))}
        </Select>
      </div>
      <div className="-mx-2 mt-3 flex max-h-80 flex-col overflow-y-auto">
        {docs.isFetching && <Loader2 className="m-3 size-4 animate-spin text-fg-subtle" />}
        {docs.error && <p className="px-2 py-2 text-sm text-bad">{errorMessage(docs.error)}</p>}
        {docs.data?.map((d) => (
          <button
            key={d.id}
            type="button"
            onClick={() => link(d.id)}
            className="flex cursor-pointer items-center gap-2.5 rounded-lg px-2 py-2 text-left text-sm hover:bg-surface-2"
          >
            <FileText className="size-4 shrink-0 text-fg-subtle" />
            <span className="min-w-0 flex-1 truncate">{d.title || d.originalFileName}</span>
            <span className="text-xs text-fg-subtle tabular-nums">{d.created.slice(0, 10)}</span>
          </button>
        ))}
        {docs.data?.length === 0 && <p className="px-2 py-2 text-sm text-fg-subtle">{t("documents.noDocs")}</p>}
      </div>
    </Dialog>
  );
};
