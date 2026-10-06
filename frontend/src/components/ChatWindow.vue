<template>
  <div class="chat-window">
    <div ref="messagesEl" class="messages-container">
      <p v-if="chatStore.activeMessages.length === 0" class="status thread-start">No messages yet.</p>
      <TransitionGroup
        :key="`${chatStore.activeChatType}-${chatStore.activeChatID}`"
        tag="div"
        name="msg"
        class="message-list"
      >
        <!-- Keyed on the sender's temp_id when there is one: the server echoes it back, so the optimistic bubble and its confirmation are one element. -->
        <div
          v-for="msg in chatStore.activeMessages"
          :key="msg.temp_id ? `${msg.sender_id}:${msg.temp_id}` : msg.id"
          :class="['message', { own: isOwn(msg), pending: msg.pending, failed: msg.failed }]"
        >
          <div class="message-bubble">
            <p>{{ msg.content }}</p>
          </div>
          <span class="timestamp">
            <time :datetime="msg.created_at">{{ formatTime(msg.created_at) }}</time>
            <Transition name="status">
              <span v-if="msg.failed" key="failed" class="send-status">Failed to send</span>
              <span v-else-if="msg.pending" key="pending" class="send-status">Sending...</span>
            </Transition>
          </span>
        </div>
      </TransitionGroup>
    </div>

    <TypingIndicator class="thread-typing" :names="typingNames" />

    <p v-if="composeDisabledReason" class="compose-disabled">
      <NavIcon name="lock" />
      <span>{{ composeDisabledReason }}</span>
    </p>
    <form v-else class="input-bar" @submit.prevent="handleSend">
      <slot name="compose-lead"></slot>

      <label for="chat-compose" class="visually-hidden">Message</label>
      <input
        id="chat-compose"
        v-model="text"
        class="text-input"
        autocomplete="off"
        maxlength="2000"
        placeholder="Type a message..."
        @input="onInput($event.target.value)"
      />
      <EmojiButton target="chat-compose" />
      <button type="submit" class="send-button">
        <span>Send</span>
        <NavIcon name="back" class="send-icon" />
      </button>
    </form>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { useTypingIndicator } from '../composables/useTypingIndicator'
import { useAuthStore } from '../stores/auth'
import { useChatStore } from '../stores/chat'
import EmojiButton from './EmojiButton.vue'
import NavIcon from './NavIcon.vue'
import TypingIndicator from './TypingIndicator.vue'

const props = defineProps({
  composeDisabledReason: { type: String, default: null },
  nameOf: { type: Function, default: null },
})

const auth = useAuthStore()
const chatStore = useChatStore()
const text = ref('')
const messagesEl = ref(null)

const { typingUserIds, onInput, onSent } = useTypingIndicator(
  () => chatStore.activeChatType,
  () => chatStore.activeChatID,
)
const typingNames = computed(() => typingUserIds.value.map((id) => props.nameOf?.(id) || 'Someone'))

function handleSend() {
  if (!text.value.trim() || !auth.user) return
  chatStore.sendMessage(
    chatStore.activeChatID,
    text.value,
    chatStore.activeChatType,
    auth.user.id,
  )
  onSent()
  text.value = ''
}

function isOwn(msg) {
  return msg.sender_id === auth.user?.id
}

function formatTime(value) {
  const date = new Date(value)
  const time = date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
  const today = new Date()
  if (date.toDateString() === today.toDateString()) return time

  const day = date.toLocaleDateString([], {
    month: 'short',
    day: 'numeric',
    ...(date.getFullYear() !== today.getFullYear() ? { year: 'numeric' } : {}),
  })
  return `${day}, ${time}`
}

watch(
  () => [chatStore.activeChatType, chatStore.activeChatID, chatStore.activeMessages.length],
  async () => {
    await nextTick()
    const el = messagesEl.value
    if (el) el.scrollTop = el.scrollHeight
  },
  { immediate: true },
)

// Measured before the DOM updates, while the indicator's height isn't in it yet.
watch(
  () => typingNames.value.length,
  async () => {
    const el = messagesEl.value
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80
    await nextTick()
    if (atBottom) el.scrollTop = el.scrollHeight
  },
)
</script>

<style scoped src="../styles/controls.css"></style>
<style scoped>
.chat-window {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.messages-container {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  padding: 22px 28px;
}

.message-list {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  animation: thread-in var(--duration-base) var(--ease-out);
}

@keyframes thread-in {
  from {
    opacity: 0;
  }
}

.thread-start {
  margin: auto;
}

.thread-typing {
  padding: 0 28px;
}

.thread-typing:deep(.typing-indicator) {
  padding-bottom: 10px;
}

.message {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  max-width: min(34rem, 78%);
  align-self: flex-start;
}

.message.own {
  align-items: flex-end;
  align-self: flex-end;
}

.message-bubble {
  padding: 10px 16px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: 20px 20px 20px 6px;
  box-shadow: 0 6px 16px -12px color-mix(in srgb, var(--primary) 40%, transparent);
  color: var(--text);
  transition:
    background-color var(--duration-base) var(--ease-out),
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    color var(--duration-base) var(--ease-out),
    opacity var(--duration-base) var(--ease-out);
}

.message-bubble p {
  margin: 0;
  font-size: var(--text-body);
  line-height: 1.45;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.own .message-bubble {
  background-color: var(--primary);
  border-color: var(--primary);
  border-radius: 20px 20px 6px 20px;
  box-shadow: 0 10px 20px -14px color-mix(in srgb, var(--primary) 80%, transparent);
  color: var(--surface);
}

.own.pending .message-bubble {
  opacity: 0.6;
}

.own.failed .message-bubble {
  background-color: var(--danger-wash);
  border-color: var(--danger-border);
  box-shadow: none;
  color: var(--danger-text-on-wash);
}

.timestamp {
  display: flex;
  gap: var(--space-2);
  padding-inline: 6px;
  font-size: 0.75rem;
  color: var(--text-placeholder);
  font-variant-numeric: tabular-nums;
}

.send-status {
  color: var(--text-muted);
  font-weight: 600;
}

.failed .send-status {
  color: var(--danger);
}

.status-enter-active,
.status-leave-active {
  transition: opacity var(--duration-base) var(--ease-out);
}

.status-enter-from,
.status-leave-to {
  opacity: 0;
}

.msg-enter-active {
  transition:
    opacity 320ms var(--ease-out),
    transform 320ms var(--ease-out);
}

.msg-enter-from {
  opacity: 0;
  transform: translateY(14px) scale(0.96);
}

.message.msg-enter-from {
  transform-origin: left bottom;
}

.message.own.msg-enter-from {
  transform-origin: right bottom;
}

.input-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 28px 18px;
  border-top: 1px solid var(--border-hairline);
}

.input-bar .text-input {
  flex: 1;
  min-width: 0;
  min-height: 3rem;
  padding: 0 20px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-pill);
  box-shadow: 0 6px 18px -12px color-mix(in srgb, var(--primary) 30%, transparent);
  font-size: var(--text-body);
  transition:
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out);
}

.input-bar .text-input::placeholder {
  color: var(--text-placeholder);
}

.input-bar .text-input:hover {
  border-color: var(--border);
}

.input-bar .text-input:focus-visible {
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
  outline: none;
  box-shadow:
    0 0 0 4px color-mix(in srgb, var(--primary) 14%, transparent),
    0 6px 18px -12px color-mix(in srgb, var(--primary) 30%, transparent);
}

.send-button {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 6px;
  min-height: 3rem;
  padding: 0 20px 0 22px;
  background-color: var(--primary);
  border: 0;
  border-radius: var(--radius-pill);
  box-shadow: var(--shadow-primary);
  color: var(--surface);
  font-size: var(--text-sm);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.send-button:hover {
  background-color: var(--primary-hover);
  box-shadow: var(--shadow-primary-lift);
  transform: translateY(-2px);
}

.send-button:active {
  box-shadow: var(--shadow-primary);
  transform: translateY(0);
  transition-duration: 80ms;
}

.send-button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 3px;
}

.send-icon {
  width: 1rem;
  height: 1rem;
  transform: rotate(180deg);
  transition: transform var(--duration-base) var(--ease-out);
}

.send-button:hover .send-icon {
  transform: rotate(180deg) translateX(-3px);
}

.compose-disabled {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  margin: 14px 28px 18px;
  padding: 12px 18px;
  background-color: var(--badge-muted-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-muted-fg);
  font-size: var(--text-sm);
  font-weight: 600;
  text-align: center;
}

.compose-disabled .nav-icon {
  flex: none;
  width: 1rem;
  height: 1rem;
}

@media (max-width: 47.99rem) {
  .messages-container {
    padding: 16px;
  }

  .thread-typing {
    padding: 0 16px;
  }

  .input-bar {
    padding: 10px 12px 12px;
  }

  .send-button {
    padding: 0 16px;
  }

  .compose-disabled {
    margin: 10px 12px 12px;
    border-radius: var(--radius-sm);
  }
}

@media (prefers-reduced-motion: reduce) {
  .message-list {
    animation: none;
  }

  .message-bubble,
  .msg-enter-active,
  .status-enter-active,
  .status-leave-active,
  .input-bar .text-input,
  .send-button,
  .send-icon {
    transition-duration: 1ms;
  }

  .send-button:hover {
    transform: none;
  }
}
</style>
