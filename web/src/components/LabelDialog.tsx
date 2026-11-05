import { Plus, Printer, QrCode as QrIcon, Tag, Unlink } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { api, errorMessage, pathText, type Thing } from "@/lib/api";
import { codeFrom, labelUrl, printList } from "@/lib/labels";
import { useInvalidate } from "@/lib/queries";
import { QrCode } from "./QrCode";
import { Button } from "./ui/button";
import { Dialog } from "./ui/dialog";
import { Input } from "./ui/input";

/** Shows the label of a thing, and makes, replaces or removes it. */
export const LabelDialog = ({
  open,
  onOpenChange,
  of,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  of: Thing;
}) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const invalidate = useInvalidate();
  const [typing, setTyping] = useState(false);
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);

  const queued = (code: string) => ({ code, name: of.name, detail: pathText(of.path) });

  const attach = async (code = "") => {
    setBusy(true);
    try {
      await api.attachLabel(of.id, code);
      invalidate();
      setTyping(false);
      setTyped("");
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  const detach = async () => {
    setBusy(true);
    try {
      await api.detachLabel(of.label);
      invalidate();
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  const typedCode = codeFrom(typed);

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("label.title", { name: of.name })}
      size="sm"
      footer={
        of.label ? (
          <>
            <Button variant="ghost" className="mr-auto" onClick={detach} disabled={busy}>
              <Unlink /> {t("label.remove")}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                printList.add([queued(of.label)]);
                toast.success(t("label.queued"));
              }}
            >
              <Plus /> {t("label.addToList")}
            </Button>
            <Button
              onClick={() => {
                printList.add([queued(of.label)]);
                void navigate("/labels");
              }}
            >
              <Printer /> {t("label.print")}
            </Button>
          </>
        ) : undefined
      }
    >
      {of.label ? (
        <div className="flex flex-col items-center gap-3 text-center">
          <QrCode text={labelUrl(of.label)} className="size-44 rounded-lg border border-border" />
          <p className="font-mono text-2xl font-semibold tracking-[0.2em]">{of.label}</p>
          <p className="text-xs text-fg-subtle">{t("label.scanHint")}</p>
        </div>
      ) : (
        <div className="flex flex-col items-center gap-3 py-2 text-center">
          <span className="flex size-12 items-center justify-center rounded-2xl bg-primary-soft text-primary">
            <Tag className="size-6" />
          </span>
          <p className="max-w-xs text-sm text-fg-muted">{t("label.none")}</p>
          <Button onClick={() => attach()} loading={busy}>
            <QrIcon /> {t("label.make")}
          </Button>
        </div>
      )}
      <div className="mt-5 border-t border-border pt-4">
        {typing ? (
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (typedCode) void attach(typedCode);
            }}
          >
            <Input
              data-autofocus
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              placeholder={t("label.codePlaceholder")}
              className="font-mono uppercase"
              autoCapitalize="characters"
            />
            <Button type="submit" variant="outline" disabled={!typedCode} loading={busy}>
              {t("label.use")}
            </Button>
          </form>
        ) : (
          <button
            type="button"
            onClick={() => setTyping(true)}
            className="cursor-pointer text-sm text-fg-muted hover:text-primary"
          >
            {of.label ? t("label.useAnother") : t("label.usePrinted")}
          </button>
        )}
      </div>
    </Dialog>
  );
};
