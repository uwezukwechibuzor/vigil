import type { Metadata } from "next";
import "./globals.css";
import { Sidebar } from "../components/layout/sidebar";
import { ThemeProvider } from "../components/providers";

export const metadata: Metadata = {
  title: "Vigil — Reliability layer for onchain AI agents",
  description: "Monitor chains, endpoints and agent runs. Run pre-transaction flight checks.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <div className="flex h-screen bg-slate-50 dark:bg-slate-950">
            <Sidebar />
            <main className="flex-1 overflow-auto p-6" id="main-content">
              {children}
            </main>
          </div>
        </ThemeProvider>
      </body>
    </html>
  );
}
