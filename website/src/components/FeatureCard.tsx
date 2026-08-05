import type { LucideIcon } from "lucide-react";

type FeatureCardProps = {
  icon: LucideIcon;
  title: string;
  children: string;
};

export function FeatureCard({ icon: Icon, title, children }: FeatureCardProps) {
  return (
    <article className="feature-card">
      <div className="feature-icon">
        <Icon size={20} aria-hidden="true" />
      </div>
      <h3>{title}</h3>
      <p>{children}</p>
    </article>
  );
}
