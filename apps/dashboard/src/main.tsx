import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { I18nProvider } from "./providers/I18nProvider";
import { SWRProvider } from "./providers/SWRProvider";
import { ThemeProvider } from "./providers/ThemeProvider";
// Side-effect import: i18next initializes on first import. Done in
// main so the catalog is ready before any component renders.
import "@/lib/i18n";
import "@/styles/index.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
	<React.StrictMode>
		<ThemeProvider>
			<I18nProvider>
				<SWRProvider>
					<BrowserRouter>
						<ErrorBoundary>
							<App />
						</ErrorBoundary>
					</BrowserRouter>
				</SWRProvider>
			</I18nProvider>
		</ThemeProvider>
	</React.StrictMode>,
);
