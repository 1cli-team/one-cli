import type React from "react";
import { type RouteObject, useRoutes } from "react-router-dom";
import { Home } from "@/pages/Home";

const routes: RouteObject[] = [
	{ path: "/", element: <Home /> },
	{ path: "*", element: <div>404 - 页面未找到</div> },
];

export const AppRoutes: React.FC = () => useRoutes(routes);
