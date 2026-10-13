import React from "react";
import ReactDOM from "react-dom/client";
import { HashRouter } from "react-router-dom";
import { DesktopGate } from "./lib/desktop/DesktopGate";
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
			<DesktopGate>
				<I18nProvider>
					<SWRProvider>
						<HashRouter>
							<ErrorBoundary>
								<App />
							</ErrorBoundary>
						</HashRouter>
					</SWRProvider>
				</I18nProvider>
			</DesktopGate>
		</ThemeProvider>
	</React.StrictMode>,
);
