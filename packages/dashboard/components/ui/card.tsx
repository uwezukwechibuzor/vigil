import { clsx } from "clsx";

interface CardProps {
  className?: string;
  children: React.ReactNode;
}

export function Card({ className, children }: CardProps) {
  return (
    <div className={clsx("rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-700 dark:bg-slate-900", className)}>
      {children}
    </div>
  );
}

export function CardHeader({ className, children }: CardProps) {
  return (
    <div className={clsx("border-b border-slate-200 px-6 py-4 dark:border-slate-700", className)}>
      {children}
    </div>
  );
}

export function CardContent({ className, children }: CardProps) {
  return <div className={clsx("px-6 py-4", className)}>{children}</div>;
}
