import React from "react";
import ReactDOM from "react-dom/client";
import { createHashRouter, RouterProvider } from "react-router-dom";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { SWRProvider } from "./providers/SWRProvider";
import { routes } from "./router/routes";
import { DesktopGate } from "./lib/desktop/DesktopGate";
import { I18nProvider } from "./providers/I18nProvider";
import { ThemeProvider } from "./providers/ThemeProvider";
import "@/lib/i18n";
import "@/styles/index.css";

const router = createHashRouter(routes);

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
	<React.StrictMode>
		<ThemeProvider>
			<DesktopGate>
				<I18nProvider>
					<SWRProvider>
						<ErrorBoundary>
							<RouterProvider router={router} />
						</ErrorBoundary>
					</SWRProvider>
				</I18nProvider>
			</DesktopGate>
		</ThemeProvider>
	</React.StrictMode>,
);
