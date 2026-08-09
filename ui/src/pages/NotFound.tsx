import { Link } from 'react-router-dom'
import { Button } from '@/components/ui/Button.tsx'

export function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <p className="tnum text-5xl font-semibold tracking-[-0.03em] text-text-muted">404</p>
      <p className="mt-2 text-[13px] text-text-secondary">Такой страницы нет</p>
      <Link to="/" className="mt-5">
        <Button variant="secondary">На дашборд</Button>
      </Link>
    </div>
  )
}
