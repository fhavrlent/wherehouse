import type { TFunction } from "i18next";
import type { KindRef } from "./api";

/** A kind's name: its own, or a built-in kind's in the reader's language. */
export const kindName = (k: Pick<KindRef, "key" | "name">, t: TFunction) =>
  k.name || (k.key ? t(`kinds.${k.key}` as "kinds.thing") : "");
