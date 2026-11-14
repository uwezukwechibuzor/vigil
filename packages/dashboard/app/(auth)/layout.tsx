import type { Metadata } from "next";
import "../globals.css";
import { ThemeProvider } from "../../components/providers";

export const metadata: Metadata = {
  title: "Vigil",
};

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <div className="flex min-h-screen items-center justify-center bg-slate-50 dark:bg-slate-950 px-4">
            {children}
          </div>
        </ThemeProvider>
      </body>
    </html>
  );
}
