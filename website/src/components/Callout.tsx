import { AlertTriangle, CheckCircle2, Info } from "lucide-react";
import type { ReactNode } from "react";

type CalloutTone = "info" | "warning" | "success";

type CalloutProps = {
  title: string;
  children: ReactNode;
  tone?: CalloutTone;
};

export function Callout({ title, children, tone = "info" }: CalloutProps) {
  const Icon = tone === "warning" ? AlertTriangle : tone === "success" ? CheckCircle2 : Info;
  return (
    <aside className={`callout callout-${tone}`}>
      <Icon size={20} aria-hidden="true" />
      <div>
        <strong>{title}</strong>
        <div>{children}</div>
      </div>
    </aside>
  );
}
