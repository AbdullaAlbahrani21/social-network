import { BASE_URL } from '../api'

export function authorName(author) {
  return author.nickname || `${author.first_name} ${author.last_name}`
}

export function authorInitials(author) {
  return `${author.first_name?.[0] ?? ''}${author.last_name?.[0] ?? ''}`.toUpperCase()
}

export function mediaUrl(path) {
  return `${BASE_URL}${path}`
}

export function formatRelativeTime(value, now = new Date()) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  const minutes = Math.floor((now - date) / 60000)
  if (minutes < 1) return 'now'
  if (minutes < 60) return `${minutes}m`

  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  if (date >= startOfToday) return `${Math.floor(minutes / 60)}h`

  const startOfYesterday = new Date(startOfToday)
  startOfYesterday.setDate(startOfYesterday.getDate() - 1)
  if (date >= startOfYesterday) return 'Yesterday'

  const startOfWeek = new Date(startOfToday)
  startOfWeek.setDate(startOfWeek.getDate() - 6)
  if (date >= startOfWeek) return date.toLocaleDateString(undefined, { weekday: 'short' })

  return date.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    ...(date.getFullYear() !== now.getFullYear() ? { year: 'numeric' } : {}),
  })
}

export function formatTimestamp(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}
