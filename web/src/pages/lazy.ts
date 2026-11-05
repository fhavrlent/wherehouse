import { lazy, type ComponentType } from "react";

// After an update, a page left open asks for code files that no longer
// exist. Reloading once fetches the new app; the flag stops a reload loop
// if the files are missing for another reason.
const page = <P extends object>(load: () => Promise<ComponentType<P>>) =>
  lazy<ComponentType<P>>(async () => {
    try {
      const component = await load();
      sessionStorage.removeItem("wherehouse-reloaded");
      return { default: component };
    } catch (e) {
      if (!sessionStorage.getItem("wherehouse-reloaded")) {
        sessionStorage.setItem("wherehouse-reloaded", "1");
        location.reload();
      }
      throw e;
    }
  });

// Pages other than the home page are downloaded when first visited.
export const Login = page(() => import("./Login").then((m) => m.Login));
export const Settings = page(() => import("./Settings").then((m) => m.Settings));
export const ThingPage = page(() => import("./Thing").then((m) => m.ThingPage));
export const PlaceRedirect = page(() => import("./Thing").then((m) => m.PlaceRedirect));
export const Join = page(() => import("./Join").then((m) => m.Join));
export const ItemsPage = page(() => import("./Items").then((m) => m.ItemsPage));
export const SearchPage = page(() => import("./Search").then((m) => m.SearchPage));
export const Labels = page(() => import("./Labels").then((m) => m.Labels));
export const Scan = page(() => import("./Scan").then((m) => m.Scan));
export const LabelPage = page(() => import("./Label").then((m) => m.LabelPage));
export const TagPage = page(() => import("./Tag").then((m) => m.TagPage));
export const Report = page(() => import("./Report").then((m) => m.Report));
export const InsuranceList = page(() => import("./Report").then((m) => m.InsuranceList));
export const Connect = page(() => import("./Connect").then((m) => m.Connect));
export const DocumentPage = page(() => import("./Document").then((m) => m.DocumentPage));
