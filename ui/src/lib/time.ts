import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime.js'

dayjs.extend(relativeTime)

export function timeAgo(date: string): string {
  return dayjs(date).fromNow()
}

export function formatDate(date: string): string {
  return dayjs(date).format('MMM D, YYYY HH:mm')
}

export function formatTime(date: string): string {
  return dayjs(date).format('HH:mm')
}

export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds} с`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} мин`
  if (seconds < 86400) {
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    return m > 0 ? `${h} ч ${m} мин` : `${h} ч`
  }
  // Дольше суток часами не меряют — иначе получается нечитаемое «4089 ч».
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  return h > 0 ? `${d} д ${h} ч` : `${d} д`
}
