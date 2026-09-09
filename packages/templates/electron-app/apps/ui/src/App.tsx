import React, { FC } from "react";
import { Link, Outlet } from "react-router-dom";

const App: FC = () => {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="border-b">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          <div>
            <h1 className="text-xl font-semibold">electron-template</h1>
            <p className="text-muted-foreground text-sm">
              electron + react 19 + tailwind v4 + shadcn
            </p>
          </div>
          <nav className="flex gap-4 text-sm">
            <Link
              to="/"
              className="hover:text-primary text-muted-foreground transition-colors"
            >
              Home
            </Link>
            <Link
              to="/about"
              className="hover:text-primary text-muted-foreground transition-colors"
            >
              About
            </Link>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  );
};

export default App;
