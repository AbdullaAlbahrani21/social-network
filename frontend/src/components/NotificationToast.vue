<script setup>
import { onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useNotificationStore } from '../stores/notifications'
import { notificationLook } from '../utils/notificationLook'
import NavIcon from './NavIcon.vue'

const TOAST_MS = 5000

const notifStore = useNotificationStore()
const route = useRoute()
const router = useRouter()

// pending is a single value, not a queue: each arrival overwrites it, so what follows a burst is the most recent thing that happened.
const showing = ref(null)
const pending = ref(null)

let timer = null
let nextKey = 0

function isViewing(notification) {
  const { type, target_id: targetID, actor_id: actorID } = notification
  const param = (name) => String(route.params[name] ?? '')

  switch (type) {
    case 'post_liked':
    case 'comment_created':
      return route.name === 'post-detail' && param('id') === String(targetID)

    case 'message_received':
      return route.name === 'chat-with' && param('userId') === String(actorID)

    case 'follow_request':
    case 'follow_accepted':
      return route.name === 'profile' && param('id') === String(actorID)

    default:
      return false
  }
}

function finish() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }

  showing.value = pending.value
  pending.value = null

  if (showing.value) timer = setTimeout(finish, TOAST_MS)
}

function openInbox() {
  finish()
  router.push('/notifications')
}

function pinOnLeave(el) {
  el.style.top = `${el.offsetTop}px`
}

notifStore.$onAction(({ name, args }) => {
  if (name !== 'receiveNotification') return

  // args, not the action's declared parameters: $onAction hands over exactly what the caller passed, so isNew is undefined when it was left off.
  const [notification, isNew = true] = args
  if (!notification) return

  if (!isNew || isViewing(notification)) return

  const toast = {
    // A key of its own, so a promoted toast is a new element and its entry animation and timer bar start over.
    key: nextKey++,
    message: notification.message || 'New notification',
    look: notificationLook(notification.type),
  }

  if (showing.value) {
    pending.value = toast
    return
  }

  showing.value = toast
  timer = setTimeout(finish, TOAST_MS)
})

onUnmounted(() => {
  if (timer) clearTimeout(timer)
  timer = null
})
</script>

<template>
  <div class="toast-stack" role="status" aria-live="polite">
    <Transition name="toast" @before-leave="pinOnLeave">
      <button
        v-if="showing"
        :key="showing.key"
        type="button"
        class="toast"
        @click="openInbox()"
      >
        <span class="toast-badge" :class="`tone-${showing.look.tone}`" aria-hidden="true">
          <NavIcon :name="showing.look.icon" class="toast-icon" />
        </span>

        <span class="toast-text">
          <span class="toast-message">{{ showing.message }}</span>
          <span class="toast-hint">View notifications</span>
        </span>

        <span class="toast-timer" aria-hidden="true">
          <span class="toast-timer-fill" :style="{ animationDuration: `${TOAST_MS}ms` }"></span>
        </span>
      </button>
    </Transition>
  </div>
</template>

<style scoped>
.toast-stack {
  position: fixed;
  left: 50%;
  bottom: var(--space-5);
  transform: translateX(-50%);
  z-index: 200;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  width: min(26rem, calc(100vw - 2 * var(--space-4)));
  pointer-events: none;
}

.toast {
  pointer-events: auto;
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  gap: var(--space-3);
  width: 100%;
  padding: 14px var(--space-4);
  padding-bottom: calc(14px + 3px);
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: 0 18px 44px -14px color-mix(in srgb, var(--primary) 42%, transparent);
  font: inherit;
  color: var(--text);
  text-align: left;
  cursor: pointer;
  transition:
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.toast:hover {
  border-color: var(--border);
  box-shadow: 0 22px 50px -14px color-mix(in srgb, var(--primary) 52%, transparent);
  transform: translateY(-2px);
}

.toast:active {
  transform: translateY(0);
  transition-duration: 80ms;
}

.toast:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.toast-badge {
  flex: none;
  display: grid;
  place-items: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: var(--radius-pill);
}

.toast-icon {
  width: 1.125rem;
  height: 1.125rem;
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

.toast-text {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.toast-message {
  font-size: var(--text-body);
  line-height: 1.4;
}

.toast-hint {
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.toast-timer {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 3px;
  background-color: var(--border);
}

.toast-timer-fill {
  display: block;
  height: 100%;
  width: 100%;
  transform-origin: left;
  background-color: var(--primary);
  animation-name: toast-drain;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}

@keyframes toast-drain {
  from { transform: scaleX(1); }
  to { transform: scaleX(0); }
}

/* Doubled class so .toast's own transition shorthand cannot outrank it and drop opacity from the list. */
.toast.toast-enter-active {
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.toast-enter-from {
  opacity: 0;
  transform: translateY(14px) scale(0.96);
}

.toast.toast-leave-active {
  position: absolute;
  right: 0;
  left: 0;
  pointer-events: none;
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.toast-leave-to {
  opacity: 0;
  transform: translateY(10px) scale(0.97);
}

@media (prefers-reduced-motion: reduce) {
  .toast-timer-fill {
    animation: none;
  }

  .toast,
  .toast.toast-enter-active,
  .toast.toast-leave-active {
    transition-duration: 1ms;
  }

  .toast:hover {
    transform: none;
  }
}

@media (max-width: 47.99rem) {
  .toast-stack {
    bottom: calc(var(--tabbar-height) + env(safe-area-inset-bottom) + var(--space-3));
  }
}
</style>
