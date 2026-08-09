import { Inbox } from 'lucide-react'
import type { ReactNode } from 'react'

export function EmptyState({ icon, title, description }: { icon?: ReactNode; title: string; description?: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="mb-3 text-text-muted">{icon || <Inbox size={40} />}</div>
      <p className="text-sm font-medium text-text-secondary">{title}</p>
      {description && <p className="mt-1 text-xs text-text-muted">{description}</p>}
    </div>
  )
}
