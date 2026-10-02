import { StrictMode } from "react";
import ReactDOM from "react-dom/client";
import { HashRouter, Route, Routes } from "react-router-dom";
import { SWRConfig } from "swr";
import App from "@/App";
import { Toaster } from "@/components/ui/toast";
import { fetcher } from "@/lib/http";
import HomePage from "@/nodes/HomePage";
import "@/styles/globals.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <StrictMode>
    <SWRConfig value={{ fetcher }}>
      <Toaster>
        <HashRouter>
          <Routes>
            <Route path="/" element={<App />}>
              <Route index element={<HomePage />} />
              <Route
                path="*"
                element={<p>404 · 页面未找到 / Page not found</p>}
              />
            </Route>
          </Routes>
        </HashRouter>
      </Toaster>
    </SWRConfig>
  </StrictMode>,
);
