import {
  Archive,
  ArchiveRestore,
  Copy,
  LayoutTemplate,
  BarChart3,
  FolderInput,
  MoreHorizontal,
  Pencil,
  Plus,
  Printer,
  QrCode,
  Trash2,
} from "lucide-react";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { Documents } from "@/components/Documents";
import { ThingFormDialog } from "@/components/Forms";
import { KindIcon } from "@/components/KindIcon";
import { LabelDialog } from "@/components/LabelDialog";
import { Links } from "@/components/Links";
import { MaintenanceLog } from "@/components/MaintenanceLog";
import { Photos } from "@/components/Photos";
import { PlacePicker } from "@/components/PlacePicker";
import { TagChip } from "@/components/Tags";
import { Breadcrumbs, Contents } from "@/components/Things";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Menu, MenuContent, MenuItem, MenuSeparator, MenuTrigger } from "@/components/ui/menu";
import { Card } from "@/components/ui/misc";
import { api, errorMessage, isPlace, pathText, type FieldType, type ThingDetail } from "@/lib/api";
import { appName } from "@/lib/apps";
import { kindName } from "@/lib/kinds";
import { printList } from "@/lib/labels";
import { useHousehold, useInvalidate, useThing } from "@/lib/queries";
import { useQuickAdd } from "@/lib/quickAdd";
import { formatMoney } from "@/lib/utils";

export const ThingPage = () => {
  const id = Number(useParams().id);
  const { data: thing, isLoading, error } = useThing(id);
  if (error) return <p className="text-sm text-fg-muted">{errorMessage(error)}</p>;
  if (isLoading || !thing) return <div className="h-96 animate-pulse rounded-2xl bg-surface-3/60" />;
  return <ThingView key={thing.id} thing={thing} />;
};

const ThingView = ({ thing }: { thing: ThingDetail }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const invalidate = useInvalidate();
  const quickAdd = useQuickAdd();
  const { data: household } = useHousehold();
  const [editing, setEditing] = useState(false);
  const [addingPlace, setAddingPlace] = useState(false);
  const [moving, setMoving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [labelling, setLabelling] = useState(false);
  const place = isPlace(thing);
  // The app whose purchase and warranty details these are, if not this one.
  const owner = thing.purchaseOwner ? thing.links?.find((l) => l.source === thing.purchaseOwner) : undefined;
  const money = (n: number) => formatMoney(n, thing.currency || household?.currency || "EUR");

  const run = async (fn: () => Promise<unknown>, done?: string) => {
    try {
      await fn();
      if (done) toast.success(done);
    } catch (e) {
      toast.error(errorMessage(e));
    }
    invalidate();
  };

  // Labels for the things directly inside, made where missing.
  const labelInside = async () => {
    try {
      const detail = pathText([...thing.path, { id: thing.id, name: thing.name, icon: thing.kind.icon }]);
      const labels = await Promise.all(
        thing.contents.map(async (c) => ({ code: (await api.attachLabel(c.id)).code, name: c.name, detail })),
      );
      printList.add(labels);
      invalidate();
      toast.success(t("label.queuedMany", { count: labels.length }), {
        action: { label: t("label.openList"), onClick: () => void navigate("/labels") },
      });
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  const details: [string, ReactNode][] = [
    [t("thingForm.quantity"), thing.quantity > 1 ? thing.quantity : ""],
    [t("thingForm.manufacturer"), thing.manufacturer],
    [t("thingForm.model"), thing.model],
    [
      t("thingForm.serial"),
      thing.serial && (
        <span key="serial" className="font-mono text-[13px]">
          {thing.serial}
        </span>
      ),
    ],
    ...(thing.fields ?? []).map((f): [string, ReactNode] => [
      f.name,
      <FieldValueView key={f.fieldId} type={f.type} value={f.value} />,
    ]),
  ];
  const purchase: [string, ReactNode][] = [
    [t("thingForm.purchaseDate"), thing.purchaseDate ?? ""],
    [
      t("thingForm.price"),
      thing.purchasePrice !== null &&
        (thing.quantity > 1
          ? t("thing.priceEach", {
              price: money(thing.purchasePrice),
              total: money(thing.purchasePrice * thing.quantity),
            })
          : money(thing.purchasePrice)),
    ],
    [t("thingForm.purchaseFrom"), thing.purchaseFrom],
    [t("thingForm.warrantyUntil"), thing.lifetimeWarranty ? t("thing.lifetime") : (thing.warrantyUntil ?? "")],
    ...detailRows(t("thingForm.warrantyDetails"), thing.warrantyDetails),
    [t("thingForm.insured"), thing.insured && t("item.yes")],
    [t("thingForm.soldDate"), thing.soldDate ?? ""],
    [t("thingForm.soldPrice"), thing.soldPrice !== null && money(thing.soldPrice)],
    [t("thingForm.soldTo"), thing.soldTo],
    [t("thingForm.soldNotes"), lines(thing.soldNotes)],
  ];
  const shown = (rows: [string, ReactNode][]) => rows.filter(([, v]) => v !== "" && v !== null && v !== false);

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex flex-col gap-3">
        <Breadcrumbs path={thing.path} current={thing.name} />
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary-soft text-primary [&>*]:size-5">
              <KindIcon icon={thing.kind.icon} />
            </span>
            <div className="min-w-0">
              <h1 className="truncate text-2xl font-semibold tracking-tight">{thing.name}</h1>
              <p className="flex flex-wrap items-center gap-1.5 text-xs text-fg-subtle">
                {kindName(thing.kind, t)}
                {thing.archivedAt && <span className="rounded bg-surface-3 px-1.5 py-0.5">{t("item.archived")}</span>}
                {thing.soldDate && <span className="rounded bg-surface-3 px-1.5 py-0.5">{t("thing.sold")}</span>}
                {thing.template && (
                  <span className="rounded bg-primary-soft px-1.5 py-0.5 text-primary">{t("thing.template")}</span>
                )}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            {place && (
              <Button onClick={() => quickAdd(thing.id)}>
                <Plus /> {t("place.addThing")}
              </Button>
            )}
            <Button variant="outline" onClick={() => setEditing(true)}>
              <Pencil /> {t("item.edit")}
            </Button>
            <Button variant="outline" onClick={() => setMoving(true)}>
              <FolderInput /> {t("item.move")}
            </Button>
            <Button variant="outline" onClick={() => setLabelling(true)}>
              <QrCode /> {thing.label || t("label.button")}
            </Button>
            <Menu>
              <MenuTrigger asChild>
                <Button variant="outline" size="icon" aria-label={t("item.more")}>
                  <MoreHorizontal />
                </Button>
              </MenuTrigger>
              <MenuContent>
                <MenuItem icon={<Plus />} onSelect={() => setAddingPlace(true)}>
                  {t("place.addPlace")}
                </MenuItem>
                {thing.contents.length > 0 && (
                  <MenuItem icon={<BarChart3 />} onSelect={() => navigate(`/report?in=${thing.id}`)}>
                    {t("report.inside")}
                  </MenuItem>
                )}
                {thing.contents.length > 0 && (
                  <MenuItem icon={<Printer />} onSelect={labelInside}>
                    {t("place.labelInside")}
                  </MenuItem>
                )}
                {!thing.template && (
                  <MenuItem
                    icon={<Copy />}
                    onSelect={() =>
                      run(async () => {
                        const c = await api.copyThing(thing.id, { sameParent: true, photos: true });
                        void navigate(`/i/${c.id}`);
                      }, t("thing.copied"))
                    }
                  >
                    {t("thing.copy")}
                  </MenuItem>
                )}
                {!thing.template && (
                  <MenuItem
                    icon={<LayoutTemplate />}
                    onSelect={() =>
                      run(() => api.copyThing(thing.id, { template: true, photos: true }), t("thing.templateSaved"))
                    }
                  >
                    {t("thing.saveTemplate")}
                  </MenuItem>
                )}
                {thing.archivedAt ? (
                  <MenuItem icon={<ArchiveRestore />} onSelect={() => run(() => api.archive(thing.id, false))}>
                    {t("item.unarchive")}
                  </MenuItem>
                ) : (
                  <MenuItem
                    icon={<Archive />}
                    onSelect={() => run(() => api.archive(thing.id, true), t("item.archivedToast"))}
                  >
                    {t("item.archive")}
                  </MenuItem>
                )}
                {thing.contents.length === 0 && (
                  <>
                    <MenuSeparator />
                    <MenuItem icon={<Trash2 />} danger onSelect={() => setDeleting(true)}>
                      {t("item.delete")}
                    </MenuItem>
                  </>
                )}
              </MenuContent>
            </Menu>
          </div>
        </div>
        {thing.tags.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {thing.tags.map((tag) => (
              <TagChip key={tag.id} tag={tag} />
            ))}
          </div>
        )}
      </div>

      <Photos id={thing.id} photos={thing.photos ?? []} />

      <Contents things={thing.contents} showPath={false} />
      {place && thing.contents.length === 0 && (
        <Card className="px-6 py-8 text-center text-sm text-fg-muted">{t("place.emptyBody")}</Card>
      )}

      {(shown(details).length > 0 || shown(purchase).length > 0 || thing.description || thing.notes) && (
        <div className="grid gap-4 md:grid-cols-2">
          {(shown(details).length > 0 || thing.description) && (
            <Card className="p-5">
              <Details rows={shown(details)} />
              {thing.description && (
                <p className="mt-4 border-t border-border pt-4 text-sm whitespace-pre-line first:mt-0 first:border-0 first:pt-0">
                  {thing.description}
                </p>
              )}
            </Card>
          )}
          {shown(purchase).length > 0 && (
            <Card className="p-5">
              <h2 className="mb-3 flex items-center justify-between gap-2 text-xs font-semibold text-fg-muted">
                {t("thingForm.purchase")}
                {owner && (
                  <a
                    href={owner.url}
                    target="_blank"
                    rel="noreferrer"
                    className="font-normal text-primary hover:underline"
                  >
                    {t("thing.fromApp", { app: appName(thing.purchaseOwner) })}
                  </a>
                )}
              </h2>
              <Details rows={shown(purchase)} />
            </Card>
          )}
          {thing.notes && (
            <Card className="p-5">
              <h2 className="text-xs font-semibold text-fg-muted">{t("thingForm.notes")}</h2>
              <p className="mt-2 text-sm whitespace-pre-line">{thing.notes}</p>
            </Card>
          )}
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-2">
        <MaintenanceLog thingId={thing.id} entries={thing.maintenance ?? []} />
        <Documents thingId={thing.id} documents={thing.attachments ?? []} />
        <Links thingId={thing.id} links={thing.links ?? []} />
      </div>

      <ThingFormDialog open={editing} onOpenChange={setEditing} thing={thing} />
      <ThingFormDialog
        open={addingPlace}
        onOpenChange={setAddingPlace}
        parentId={thing.id}
        kindKey={thing.kind.key === "building" ? "room" : "furniture"}
        onSaved={(p) => void navigate(`/i/${p.id}`)}
      />
      <LabelDialog open={labelling} onOpenChange={setLabelling} of={thing} />
      <PlacePicker
        open={moving}
        onOpenChange={setMoving}
        title={t("item.moveTitle", { name: thing.name })}
        value={thing.parentId}
        exclude={thing.id}
        noneLabel={t("thing.nowhere")}
        onPick={(into) => run(() => api.move(into, [thing.id]), t("move.done"))}
      />
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={t("item.deleteTitle", { name: thing.name })}
        description={t("item.deleteBody")}
        onConfirm={async () => {
          try {
            await api.deleteThing(thing.id);
            invalidate();
            void navigate(thing.parentId ? `/i/${thing.parentId}` : "/");
          } catch (e) {
            toast.error(errorMessage(e));
          }
        }}
      />
    </div>
  );
};

const Details = ({ rows }: { rows: [string, ReactNode][] }) => (
  <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2.5 text-sm">
    {rows.map(([k, v]) => (
      <div key={k} className="contents">
        <dt className="text-fg-muted">{k}</dt>
        <dd className="min-w-0 break-words">{v}</dd>
      </div>
    ))}
  </dl>
);

const FieldValueView = ({ type, value }: { type: FieldType; value: string }) => {
  const { t } = useTranslation();
  switch (type) {
    case "bool":
      return <>{t("item.yes")}</>;
    case "url":
      return (
        <a href={value} target="_blank" rel="noreferrer" className="break-all text-primary hover:underline">
          {value.replace(/^https?:\/\//, "")}
        </a>
      );
    default:
      return <>{value}</>;
  }
};

/** Old place links lead to the same page. */
export const PlaceRedirect = () => <Navigate to={`/i/${useParams().id ?? ""}`} replace />;

/**
 * Rows for text that may have several lines. A line like "Order number: 123"
 * (as Warranteed writes them) is a row of its own, with its label beside the
 * others; the rest stays under label.
 */
const detailRows = (label: string, text: string): [string, ReactNode][] => {
  const rest: string[] = [];
  const own: [string, ReactNode][] = [];
  for (const line of text.split("\n")) {
    const [, name, value] = /^([^:]{1,40}):\s+(.+)$/.exec(line.trim()) ?? [];
    if (name && value && !/^https?$/i.test(name)) own.push([name, value]);
    else if (line.trim()) rest.push(line);
  }
  return [[label, lines(rest.join("\n"))], ...own];
};

/** Text that may have several lines, kept as they are; "" when there's none. */
const lines = (text: string) => text && <span className="whitespace-pre-line">{text}</span>;
