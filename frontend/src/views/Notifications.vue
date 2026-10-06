<template>
  <div class="notifications-page">
    <div class="page-head">
      <h1>All Notifications</h1>
      <button
        type="button"
        class="mark-all"
        :disabled="notifStore.unreadCount === 0"
        @click="markAllAsRead()"
      >
        Mark all as read
      </button>
    </div>

    <p v-if="notifStore.isLoading" class="page-status loading-status">Loading...</p>
    <p v-else-if="notifStore.notifications.length === 0" class="page-status empty">
      No notifications found.
    </p>

    <ul v-else class="full-notif-list" :class="{ 'is-clearing': clearingAll }">
      <li
        v-for="(item, index) in notifStore.notifications"
        :key="item.id"
        :class="{ unread: !item.is_read }"
        :style="{ '--stagger': Math.min(index, 8) }"
      >
        <!-- A <button> may only contain phrasing content, which is why the text below is in spans rather than a <p> and a <small>. -->
        <button type="button" class="row" @click="openNotification(item)">
          <span class="type-badge" :class="`tone-${look(item).tone}`" aria-hidden="true">
            <NavIcon :name="look(item).icon" class="type-icon" />
          </span>

          <span class="row-text">
            <span class="message">{{ item.message || 'Notification event' }}</span>
            <time class="when" :datetime="item.created_at">
              {{ new Date(item.created_at).toLocaleString() }}
            </time>
          </span>

          <span v-if="!item.is_read" class="visually-hidden">Unread</span>

          <span class="unread-dot" aria-hidden="true"></span>
        </button>
      </li>
    </ul>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notifications'
import { notificationLook } from '../utils/notificationLook'
import NavIcon from '../components/NavIcon.vue'

const notifStore = useNotificationStore()
const auth = useAuthStore()
const router = useRouter()

function look(item) {
  return notificationLook(item.type)
}

// target_id is whatever that type's target happens to be -- the post for a like, the followers row for a follow request -- so routing is per type.
function destinationFor(item) {
  switch (item.type) {
    case 'post_liked':
    case 'comment_created':
      return `/posts/${item.target_id}`

    case 'message_received':
      return actorRoute(item, 'chat')

    case 'follow_accepted':
      return actorRoute(item, 'profile')

    case 'follow_request':
      return auth.user?.id ? `/profile/${auth.user.id}` : null

    default:
      return null
  }
}

// actor_id is ON DELETE SET NULL and the API renders a null actor as 0, so routing on it would open /profile/0.
function actorRoute(item, segment) {
  return item.actor_id ? `/${segment}/${item.actor_id}` : null
}

function openNotification(item) {
  // Not awaited: markAsRead clears the flag synchronously, so the highlight goes on this tick and navigation doesn't wait on the network.
  if (!item.is_read) notifStore.markAsRead(item.id)

  const to = destinationFor(item)
  if (to) router.push(to)
}

const clearingAll = ref(false)
let clearingTimer = null

// The ninth row's delay, where the stagger is capped, plus the transition itself.
const CLEAR_STAGGER_MS = 8 * 45 + 400

function markAllAsRead() {
  clearingAll.value = true
  clearTimeout(clearingTimer)
  clearingTimer = setTimeout(() => {
    clearingAll.value = false
  }, CLEAR_STAGGER_MS)

  notifStore.markAllAsRead()
}

onMounted(() => {
  notifStore.fetchNotifications()
})

onUnmounted(() => {
  clearTimeout(clearingTimer)
})
</script>

<style scoped>
.notifications-page {
  max-width: 42.5rem;
  margin: 0 auto;
  padding-block: 40px var(--space-7);
}

.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  flex-wrap: wrap;
  margin-bottom: 20px;
}

.page-head h1 {
  margin: 0;
}

.mark-all {
  min-height: 2.25rem;
  padding: 0 18px;
  background-color: color-mix(in srgb, var(--text) 6%, var(--surface));
  border: 1px solid transparent;
  border-radius: var(--radius-pill);
  color: var(--text-muted);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.mark-all:hover:not(:disabled) {
  background-color: color-mix(in srgb, var(--text) 10%, var(--surface));
  color: var(--text);
  transform: translateY(-1px);
}

.mark-all:active:not(:disabled) {
  transform: translateY(0);
  transition-duration: 80ms;
}

.mark-all:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.mark-all:disabled {
  opacity: 0.6;
  cursor: default;
}

.page-status {
  margin: 0;
  color: var(--text-muted);
}

.empty {
  padding: 34px var(--space-5);
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
  text-align: center;
}

.loading-status {
  animation: appear-late 1ms 300ms both;
}

@keyframes appear-late {
  from { opacity: 0; }
  to { opacity: 1; }
}

.full-notif-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 16px 18px;
  overflow: hidden;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    transform var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    border-color var(--duration-base) var(--ease-out);
}

.row:hover {
  border-color: var(--border);
  box-shadow: 0 18px 36px -18px color-mix(in srgb, var(--primary) 38%, transparent);
  transform: translateY(-2px);
}

.row:active {
  transform: translateY(0);
  transition-duration: 80ms;
}

.row:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.type-badge {
  flex: none;
  display: grid;
  place-items: center;
  width: 2.5rem;
  height: 2.5rem;
  border-radius: var(--radius-pill);
}

.type-icon {
  width: 1.25rem;
  height: 1.25rem;
}

.tone-purple {
  background-color: var(--badge-purple-bg);
  color: var(--badge-purple-fg);
}

.tone-pink {
  background-color: var(--badge-pink-bg);
  color: var(--badge-pink-fg);
}

.tone-muted {
  background-color: var(--badge-muted-bg);
  color: var(--badge-muted-fg);
}

.row-text {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.message {
  font-size: var(--text-body);
  line-height: 1.45;
}

.when {
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.row::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 4px;
  background-color: color-mix(in srgb, var(--primary) 45%, var(--surface));
  opacity: 0;
  transition: opacity var(--duration-base) var(--ease-out);
}

.unread .row::before {
  opacity: 1;
}

.unread-dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: var(--radius-pill);
  background-color: var(--notification-dot);
  opacity: 0;
  transform: scale(0.4);
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.unread .unread-dot {
  opacity: 1;
  transform: scale(1);
}

.unread .message {
  font-weight: 600;
}

.is-clearing .row::before,
.is-clearing .unread-dot {
  transition-delay: calc(var(--stagger, 0) * 45ms);
}

@media (max-width: 47.99rem) {
  .notifications-page {
    padding: 20px 16px 40px;
  }

  .page-head {
    margin-bottom: 16px;
  }

  .row {
    gap: 12px;
    padding: 14px 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .row,
  .row::before,
  .unread-dot,
  .mark-all {
    transition-duration: 1ms;
    transition-delay: 0ms;
  }

  .row:hover,
  .mark-all:hover:not(:disabled) {
    transform: none;
  }
}
</style>
