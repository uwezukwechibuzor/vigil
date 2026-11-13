"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { clsx } from "clsx";
import { Activity, Server, Database, AlertTriangle, Zap, Key, BarChart2, Users, Settings, Radio } from "lucide-react";

const nav = [
  { href: "/", label: "Overview", icon: Activity, id: "nav-overview" },
  { href: "/endpoints", label: "Endpoints", icon: Server, id: "nav-endpoints" },
  { href: "/oracles", label: "Oracles", icon: Database, id: "nav-oracles" },
  { href: "/flight-checks", label: "Flight Checks", icon: Zap, id: "nav-flight-checks" },
  { href: "/runs", label: "Agent Runs", icon: Radio, id: "nav-runs" },
  { href: "/incidents", label: "Incidents", icon: AlertTriangle, id: "nav-incidents" },
  { href: "/policies", label: "Policies", icon: Settings, id: "nav-policies" },
  { href: "/alert-rules", label: "Alert Rules", icon: AlertTriangle, id: "nav-alert-rules" },
  { href: "/api-keys", label: "API Keys", icon: Key, id: "nav-api-keys" },
  { href: "/usage", label: "Usage", icon: BarChart2, id: "nav-usage" },
  { href: "/settings", label: "Team", icon: Users, id: "nav-team" },
];

export function Sidebar() {
  const pathname = usePathname();
  return (
    <aside className="flex h-screen w-56 flex-col border-r border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <div className="flex h-14 items-center border-b border-slate-200 px-4 dark:border-slate-700">
        <span className="text-lg font-bold text-brand-500">Vigil</span>
      </div>
      <nav className="flex-1 overflow-y-auto p-2">
        {nav.map(({ href, label, icon: Icon, id }) => (
          <Link
            key={href}
            href={href}
            data-analytics-id={id}
            className={clsx(
              "flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors",
              pathname === href
                ? "bg-brand-50 text-brand-700 dark:bg-slate-800 dark:text-brand-400"
                : "text-slate-600 hover:bg-slate-50 dark:text-slate-400 dark:hover:bg-slate-800",
            )}
          >
            <Icon size={16} aria-hidden="true" />
            {label}
          </Link>
        ))}
      </nav>
    </aside>
  );
}
