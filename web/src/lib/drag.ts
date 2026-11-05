import { useState, type DragEvent } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api, errorMessage } from "./api";
import { useInvalidate } from "./queries";

// Things are moved by dragging them onto places.

const dragType = "application/x-wherehouse";

export const dragProps = (things: number[]) => ({
  draggable: true,
  onDragStart: (e: DragEvent) => {
    e.dataTransfer.setData(dragType, JSON.stringify(things));
    e.dataTransfer.effectAllowed = "move";
  },
});

/** Lets things be dropped onto a thing (null: onto nothing) to move them into it. */
export const useDropTarget = (into: number | null) => {
  const { t } = useTranslation();
  const invalidate = useInvalidate();
  const [over, setOver] = useState(false);
  const accepts = (e: DragEvent) => e.dataTransfer.types.includes(dragType);
  return {
    over,
    props: {
      onDragOver: (e: DragEvent) => {
        if (!accepts(e)) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        setOver(true);
      },
      onDragLeave: () => setOver(false),
      onDrop: async (e: DragEvent) => {
        setOver(false);
        if (!accepts(e)) return;
        e.preventDefault();
        const things = JSON.parse(e.dataTransfer.getData(dragType)) as number[];
        if (into !== null && things.includes(into)) return;
        try {
          await api.move(into, things);
          toast.success(t("move.done"));
        } catch (err) {
          toast.error(errorMessage(err));
        }
        invalidate();
      },
    },
  };
};
