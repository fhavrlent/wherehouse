import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Outlet, Route, Routes } from "react-router";
import { Toaster } from "sonner";
import { AppShell } from "./components/AppShell";
import { LanguageRoot } from "./components/LanguageRoot";
import { initI18n } from "./i18n";
import "./index.css";
import { queryClient } from "./lib/queries";
import { Home } from "./pages/Home";
import {
  Connect,
  DocumentPage,
  InsuranceList,
  ItemsPage,
  Join,
  LabelPage,
  Labels,
  Login,
  PlaceRedirect,
  Report,
  Scan,
  SearchPage,
  Settings,
  TagPage,
  ThingPage,
} from "./pages/lazy";

const root = document.getElementById("root");
if (!root) throw new Error("#root element missing");

await initI18n();

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <LanguageRoot>
          <Suspense fallback={null}>
            <Routes>
              <Route path="/login" element={<Login />} />
              <Route
                element={
                  <AppShell>
                    <Outlet />
                  </AppShell>
                }
              >
                <Route path="/" element={<Home />} />
                <Route path="/i/:id" element={<ThingPage />} />
                <Route path="/p/:id" element={<PlaceRedirect />} />
                <Route path="/join/:token" element={<Join />} />
                <Route path="/connect" element={<Connect />} />
                <Route path="/d/:doc" element={<DocumentPage />} />
                <Route path="/unplaced" element={<ItemsPage show="unplaced" />} />
                <Route path="/archived" element={<ItemsPage show="archived" />} />
                <Route path="/search" element={<SearchPage />} />
                <Route path="/t/:id" element={<TagPage />} />
                <Route path="/report" element={<Report />} />
                <Route path="/report/insurance" element={<InsuranceList />} />
                <Route path="/labels" element={<Labels />} />
                <Route path="/scan" element={<Scan />} />
                <Route path="/l/:code" element={<LabelPage />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="*" element={<Home />} />
              </Route>
            </Routes>
          </Suspense>
        </LanguageRoot>
      </BrowserRouter>
      <Toaster
        position="bottom-right"
        richColors
        closeButton
        theme="system"
        toastOptions={{ className: "font-sans" }}
      />
    </QueryClientProvider>
  </StrictMode>,
);
