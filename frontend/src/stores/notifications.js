import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api'

export const useNotificationStore = defineStore('notifications', () => {
  const notifications = ref([])
  const isLoading = ref(false)

  const unreadCount = computed(() => notifications.value.filter(n => !n.is_read).length)

  async function fetchNotifications() {
    isLoading.value = true
    try {
      const data = await api.get('/api/notifications')
      notifications.value = data || []
    } catch (err) {
      console.error('Error fetching notifications:', err)
    } finally {
      isLoading.value = false
    }
  }

  async function markAsRead(notificationId) {
    const item = notifications.value.find(n => n.id === notificationId)
    if (item) item.is_read = true

    try {
      await api.post(`/api/notifications/${notificationId}/read`)
    } catch (err) {
      console.error('Failed to mark notification as read:', err)
    }
  }

  async function markAllAsRead() {
    notifications.value.forEach(n => { n.is_read = true })

    try {
      await api.post('/api/notifications/read')
    } catch (err) {
      console.error('Failed to mark all notifications as read:', err)
    }
  }

  // isNew says whether the server created this notification or refreshed one already here; only the toast cares, and the row is upserted by id either way.
  function receiveNotification(newNotif, isNew = true) {
    const existing = notifications.value.findIndex(n => n.id === newNotif.id)
    if (existing !== -1) notifications.value.splice(existing, 1)

    notifications.value.unshift(newNotif)
  }

  function reset() {
    notifications.value = []
    isLoading.value = false
  }

  return { notifications, unreadCount, isLoading, fetchNotifications, markAsRead, markAllAsRead, receiveNotification, reset }
})
