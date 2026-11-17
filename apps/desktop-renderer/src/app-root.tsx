import React from "react";
import ReactDOM from "react-dom/client";
import { HashRouter } from "react-router-dom";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { SWRProvider } from "./providers/SWRProvider";
import { App } from "./App";
import { DesktopGate } from "./lib/desktop/DesktopGate";
import { I18nProvider } from "./providers/I18nProvider";
import { ThemeProvider } from "./providers/ThemeProvider";
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
