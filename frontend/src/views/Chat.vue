<template>
  <div class="messages-page" :class="{ 'thread-open': dmOpen }">
    <section class="inbox" aria-labelledby="inbox-title">
      <h1 id="inbox-title" class="inbox-title">Messages</h1>

      <div class="inbox-search" :class="{ 'is-engaged': searchEngaged }">
        <UserSearch
          inline
          label="Search people"
          :unavailable-reason="messagingUnavailable"
          :openable-anyway="hasConversationWith"
          @select="openChat"
          @engaged="searchEngaged = $event"
        />
      </div>

      <Transition
        name="list-fade"
        @before-leave="(el) => { el.style.top = `${el.offsetTop}px`; el.style.height = `${el.offsetHeight}px` }"
      >
        <div v-if="!searchEngaged" class="inbox-list">
          <p v-if="conversationsError" class="error inbox-message" role="alert">{{ conversationsError }}</p>
          <p v-else-if="!conversationsLoaded" class="status inbox-message">Loading conversations...</p>
          <p v-else-if="conversations.length === 0" class="status inbox-message">
            No conversations yet. Search for someone to start one.
          </p>

          <ul v-else class="conversation-list" aria-label="Direct messages">
            <li v-for="conversation in conversations" :key="'dm-' + conversation.userId">
              <button
                type="button"
                class="conversation"
                :class="{
                  'is-active': isActive(conversation.userId, 'dm'),
                  'is-unread': conversation.unreadCount > 0,
                  'is-muted': !conversation.canMessage,
                }"
                :aria-current="isActive(conversation.userId, 'dm') ? 'true' : undefined"
                :title="conversation.canMessage ? undefined : cantMessageExplanation"
                @click="openChat({ id: conversation.userId })"
              >
                <img
                  v-if="conversation.avatarPath && !failedAvatars.has(conversation.userId)"
                  :src="mediaUrl(conversation.avatarPath)"
                  alt=""
                  class="avatar"
                  @error="failedAvatars.add(conversation.userId)"
                />
                <span v-else class="avatar avatar-initials" aria-hidden="true">{{ initialsOf(conversation) }}</span>

                <span class="conversation-body">
                  <span class="conversation-line">
                    <span class="conversation-name">{{ conversation.nickname || conversation.name }}</span>
                    <time
                      class="conversation-time"
                      :datetime="conversation.lastMessageAt"
                      :title="formatTimestamp(conversation.lastMessageAt)"
                    >{{ formatRelativeTime(conversation.lastMessageAt, now) }}</time>
                  </span>
                  <span class="conversation-line">
                    <span class="conversation-preview">
                      <template v-if="conversation.lastSenderId === auth.user?.id">You: </template>{{ conversation.lastMessage }}
                    </span>
                    <span v-if="conversation.unreadCount > 0" class="unread-badge">
                      {{ conversation.unreadCount }}<span class="visually-hidden">&nbsp;unread</span>
                    </span>
                  </span>
                  <span v-if="!conversation.canMessage" class="conversation-note">
                    <NavIcon name="lock" />Can no longer message
                  </span>
                </span>
              </button>
            </li>
          </ul>
        </div>
      </Transition>
    </section>

    <section class="thread" aria-label="Conversation">
      <template v-if="dmOpen">
        <header class="thread-head">
          <button type="button" class="back-button" aria-label="Back to conversations" @click="closeThread">
            <NavIcon name="back" />
          </button>

          <img
            v-if="activePartner?.avatarPath && !failedAvatars.has(chatStore.activeChatID)"
            :src="mediaUrl(activePartner.avatarPath)"
            alt=""
            class="avatar"
            @error="failedAvatars.add(chatStore.activeChatID)"
          />
          <span v-else class="avatar avatar-initials" aria-hidden="true">{{ activePartner ? initialsOf(activePartner) : '?' }}</span>
          <span class="thread-who">
            <span class="thread-name">{{ activePartner ? activePartner.nickname || activePartner.name : 'Conversation' }}</span>
            <RouterLink :to="`/profile/${chatStore.activeChatID}`" class="thread-link">View profile</RouterLink>
          </span>
        </header>

        <ChatWindow :compose-disabled-reason="composeDisabledReason" :name-of="typingName" />
      </template>

      <div v-else class="thread-empty">
        <span class="thread-empty-icon"><NavIcon name="messages" /></span>
        <h2 class="thread-empty-title">Select a conversation</h2>
        <p class="status">Or search for someone to start one.</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, ref, reactive, onMounted, onUnmounted, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useChatStore } from '../stores/chat'
import { useNotificationStore } from '../stores/notifications'
import { api } from '../api'
import { formatRelativeTime, formatTimestamp, mediaUrl } from '../utils/display'
import ChatWindow from '../components/ChatWindow.vue'
import NavIcon from '../components/NavIcon.vue'
import UserSearch from '../components/UserSearch.vue'

const auth = useAuthStore()
const chatStore = useChatStore()
const notifStore = useNotificationStore()
const route = useRoute()
const router = useRouter()
const contacts = ref([])
const contactsLoaded = ref(false)

const conversations = ref([])
const conversationsLoaded = ref(false)
const conversationsError = ref('')

const searchEngaged = ref(false)

const failedAvatars = reactive(new Set())

const cantMessageExplanation =
  "You can read this conversation, but you can't send new messages: their profile is private and neither of you follows the other."

const now = ref(new Date())
let clock = null
onMounted(() => {
  clock = setInterval(() => (now.value = new Date()), 60_000)
})
onUnmounted(() => clearInterval(clock))

// Bumped on every load: the list is reloaded while the page is open, so an older response must not land on top of a newer one.
let contactsToken = 0

async function loadContacts() {
  const token = ++contactsToken
  try {
    const data = await api.get('/api/messageable-users')
    if (token !== contactsToken) return
    contacts.value = data || []
    contactsLoaded.value = true
  } catch (err) {
    if (token !== contactsToken) return
    console.error('Failed to load contacts:', err)
  }
}

let conversationsToken = 0

async function loadConversations() {
  const token = ++conversationsToken
  try {
    const data = await api.get('/api/conversations')
    if (token !== conversationsToken) return
    conversations.value = data || []
    conversationsLoaded.value = true
    conversationsError.value = ''
  } catch (err) {
    if (token !== conversationsToken) return
    if (!conversationsLoaded.value) conversationsError.value = err.message
    console.error('Failed to load conversations:', err)
  }
}

// Every send arrives twice (the HTTP reply and the WebSocket echo), so reloads are coalesced rather than sent per event.
let reloadTimer = null
function scheduleConversationsReload() {
  clearTimeout(reloadTimer)
  reloadTimer = setTimeout(loadConversations, 250)
}
onUnmounted(() => clearTimeout(reloadTimer))

chatStore.$onAction(({ name, args, after }) => {
  if (name === 'receiveMessage' && !args[0]?.group_id) {
    after(scheduleConversationsReload)
  } else if (name === 'sendMessage' && args[2] === 'dm') {
    after(scheduleConversationsReload)
  }
})

notifStore.$onAction(({ name, args }) => {
  if (name === 'receiveNotification' && args[0]?.type === 'follow_accepted') {
    loadContacts()
    loadConversations()
  }
})

function reloadOnFocus() {
  loadContacts()
  loadConversations()
}
onMounted(() => window.addEventListener('focus', reloadOnFocus))
onUnmounted(() => window.removeEventListener('focus', reloadOnFocus))

onMounted(() => {
  loadConversations()
  loadContacts()
})

// Opened once the user is known: on a hard refresh setUser arrives after this view mounts and resets the chat store, wiping a DM opened on mount.
watch(
  () => [auth.user?.id, route.params.userId],
  ([userID, param]) => {
    if (!userID) return
    const otherID = Number(param)
    if (Number.isInteger(otherID) && otherID > 0) {
      chatStore.setActiveChat(otherID, 'dm')
    }
  },
  { immediate: true },
)

function isActive(id, type) {
  return chatStore.activeChatID === id && chatStore.activeChatType === type
}

function closeThread() {
  chatStore.activeChatID = null
  if (route.params.userId) router.push('/chat')
}

const contactIDs = computed(() => new Set(contacts.value.map((contact) => contact.id)))
const conversationIDs = computed(() => new Set(conversations.value.map((c) => c.userId)))

function messagingUnavailable(user) {
  if (user.id === auth.user?.id) return 'You'
  if (contactsLoaded.value && !contactIDs.value.has(user.id)) return "Can't message"
  return null
}

function hasConversationWith(user) {
  return conversationIDs.value.has(user.id)
}

const lastChosen = ref(null)

const activePartner = computed(() => {
  if (chatStore.activeChatType !== 'dm' || !chatStore.activeChatID) return null
  const id = chatStore.activeChatID

  const conversation = conversations.value.find((c) => c.userId === id)
  if (conversation) return { id, name: conversation.name, nickname: conversation.nickname, avatarPath: conversation.avatarPath }

  if (lastChosen.value?.id === id && lastChosen.value.name) return lastChosen.value

  const contact = contacts.value.find((c) => c.id === id)
  if (contact) {
    return { id, name: `${contact.firstName} ${contact.lastName}`, nickname: contact.nickname, avatarPath: contact.avatarPath }
  }
  return null
})

function typingName(userId) {
  const partner = activePartner.value
  return partner && partner.id === userId ? partner.nickname || partner.name : null
}

const dmOpen = computed(() => chatStore.activeChatType === 'dm' && chatStore.activeChatID != null)

const composeDisabledReason = computed(() => {
  if (chatStore.activeChatType !== 'dm' || !chatStore.activeChatID) return null
  const id = chatStore.activeChatID
  const who = activePartner.value ? activePartner.value.nickname || activePartner.value.name : 'this person'

  const conversation = conversations.value.find((c) => c.userId === id)
  const canMessage = conversation
    ? conversation.canMessage
    : !contactsLoaded.value || contactIDs.value.has(id)
  return canMessage ? null : `You can no longer message ${who}.`
})

function initialsOf(person) {
  const source = person.name || ''
  const [first = '', ...rest] = source.split(' ')
  return `${first[0] ?? ''}${rest.join(' ')[0] ?? ''}`.toUpperCase() || '?'
}

function openChat(user) {
  lastChosen.value = user
  chatStore.setActiveChat(user.id, 'dm')
  router.push(`/chat/${user.id}`)
}

// markedRead keeps one message from being sent twice while its request is out; a failed request lets it retry.
const markedRead = new Set()

watch(
  () => auth.user?.id,
  () => markedRead.clear(),
)

watch(
  () => [chatStore.activeChatType, chatStore.activeChatID, chatStore.activeMessages.length, auth.user?.id],
  async () => {
    const me = auth.user?.id
    const other = chatStore.activeChatID
    if (chatStore.activeChatType !== 'dm' || !other || !me) return

    const ids = chatStore.activeMessages
      .filter((m) => m.id != null && m.sender_id === other && m.receiver_id === me && !m.read_at && !markedRead.has(m.id))
      .map((m) => m.id)
    if (ids.length === 0) return

    ids.forEach((id) => markedRead.add(id))
    try {
      await api.post('/api/messages/read', { message_ids: ids })
      scheduleConversationsReload()
    } catch (err) {
      ids.forEach((id) => markedRead.delete(id))
      console.error('Failed to mark messages read:', err)
    }
  },
)
</script>

<style scoped>
.messages-page {
  display: grid;
  grid-template-columns: minmax(18rem, 20rem) minmax(0, 1fr);
  height: 100vh;
  margin-inline: calc(var(--space-6) * -1);
  background-color: var(--bg);
}

.inbox {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background-color: var(--surface);
  border-right: 1px solid var(--border-hairline);
}

.inbox-title {
  margin: 0;
  padding: 36px 20px 16px;
  font-size: 1.75rem;
  font-weight: 800;
}

.inbox-search {
  flex: none;
  padding: 0 20px 14px;
}

.inbox-search.is-engaged {
  flex: 1;
  min-height: 0;
}

.inbox-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 12px var(--space-4);
}

.list-fade-enter-active,
.list-fade-leave-active {
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.list-fade-leave-active {
  position: absolute;
  right: 0;
  left: 0;
}

.list-fade-enter-from,
.list-fade-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

.inbox-message {
  padding: var(--space-3);
}

.conversation-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.conversation {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
  padding: 12px;
  background: none;
  border: 0;
  border-radius: 16px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
  transition: background-color var(--duration-base) var(--ease-out);
}

.conversation:hover {
  background-color: var(--bg);
}

.conversation.is-active {
  background-color: var(--badge-purple-bg);
}

.conversation:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: -2px;
}

.avatar {
  display: block;
  flex: none;
  width: 2.75rem;
  height: 2.75rem;
  border-radius: 50%;
  object-fit: cover;
  transition:
    opacity var(--duration-base) var(--ease-out),
    filter var(--duration-base) var(--ease-out);
}

.avatar-initials {
  display: grid;
  place-items: center;
  background-color: var(--badge-purple-bg);
  color: var(--badge-purple-fg);
  font-size: var(--text-label);
  font-weight: 700;
}

.is-active .avatar-initials {
  background-color: var(--surface);
}

.conversation-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
  line-height: 1.35;
}

.conversation-line {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  min-width: 0;
}

.conversation-name,
.conversation-preview {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conversation-name {
  flex: 1;
  min-width: 0;
  font-size: var(--text-sm);
  font-weight: 700;
}

.conversation-time {
  flex: none;
  color: var(--text-placeholder);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}

.conversation-preview {
  flex: 1;
  min-width: 0;
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.is-unread .conversation-name {
  font-weight: 800;
}

.is-unread .conversation-preview {
  color: var(--text);
  font-weight: 600;
}

.is-unread .conversation-time {
  color: var(--primary);
  font-weight: 700;
}

.unread-badge {
  flex: none;
  align-self: center;
  min-width: 1.3rem;
  padding: 0 6px;
  border-radius: var(--radius-pill);
  background-color: var(--notification-dot);
  color: var(--text);
  font-size: 0.7rem;
  font-weight: 800;
  line-height: 1.3rem;
  text-align: center;
  animation: badge-in var(--duration-base) var(--ease-out);
}

@keyframes badge-in {
  from {
    opacity: 0;
    transform: scale(0.6);
  }
}

.is-muted .avatar {
  opacity: 0.55;
  filter: grayscale(1);
}

.is-muted .conversation-name,
.is-muted .conversation-preview {
  color: var(--text-muted);
}

.is-muted .conversation-name {
  font-weight: 600;
}

.is-muted .conversation-time {
  color: var(--text-placeholder);
  font-weight: 400;
}

.is-muted .conversation-preview {
  font-weight: 400;
}

.conversation-note {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  width: fit-content;
  margin-top: 4px;
  padding: 2px 8px;
  background-color: var(--badge-muted-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-muted-fg);
  font-size: 0.7rem;
  font-weight: 700;
}

.conversation-note .nav-icon {
  width: 0.75rem;
  height: 0.75rem;
}

.thread {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.thread-head {
  display: flex;
  flex: none;
  align-items: center;
  gap: 12px;
  min-height: 4.75rem;
  padding: 14px 28px;
  border-bottom: 1px solid var(--border-hairline);
}

.thread-head .avatar {
  width: 2.5rem;
  height: 2.5rem;
}

.thread-who {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.3;
}

.thread-name {
  overflow: hidden;
  font-size: var(--text-card-title);
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.thread-link {
  width: fit-content;
  color: var(--primary);
  font-size: var(--text-meta);
  font-weight: 700;
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out);
}

.thread-link:hover {
  color: var(--primary-hover);
  text-decoration: underline;
  text-underline-offset: 0.2em;
}

.back-button {
  display: none;
  place-items: center;
  width: 2.5rem;
  height: 2.5rem;
  margin-left: -8px;
  background: none;
  border: 0;
  border-radius: 50%;
  color: var(--text);
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-out);
}

.back-button:hover {
  background-color: var(--badge-purple-bg);
}

.back-button .nav-icon {
  width: 1.2rem;
  height: 1.2rem;
}

.thread > .chat-window {
  flex: 1;
  min-height: 0;
}

.thread-empty {
  display: grid;
  place-content: center;
  justify-items: center;
  gap: var(--space-2);
  height: 100%;
  padding: var(--space-6);
  text-align: center;
}

.thread-empty-icon {
  display: grid;
  place-items: center;
  width: 4.5rem;
  height: 4.5rem;
  margin-bottom: var(--space-2);
  background-color: var(--badge-muted-bg);
  border-radius: 50%;
  color: var(--primary);
}

.thread-empty-icon .nav-icon {
  width: 1.5rem;
  height: 1.5rem;
}

.thread-empty-title {
  margin: 0;
  font-size: var(--text-card-title);
}

.status {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-sm);
}

.error {
  margin: 0;
  color: var(--danger);
  font-size: var(--text-sm);
}

.error::first-letter {
  text-transform: uppercase;
}

@media (max-width: 47.99rem) {
  .messages-page {
    grid-template-columns: minmax(0, 1fr);
    height: calc(100dvh - var(--topbar-height) - var(--tabbar-height) - env(safe-area-inset-bottom));
    margin-inline: 0;
  }

  .inbox {
    border-right: 0;
  }

  .messages-page.thread-open .inbox,
  .messages-page:not(.thread-open) .thread {
    display: none;
  }

  .inbox,
  .thread {
    animation: column-in var(--duration-base) var(--ease-out);
  }

  .inbox-title {
    padding: 20px 16px 12px;
    font-size: 1.5rem;
  }

  .inbox-search {
    padding: 0 16px 12px;
  }

  .inbox-list {
    padding: 0 8px 16px;
  }

  .thread-head {
    min-height: 4rem;
    padding: 10px 12px;
  }

  .back-button {
    display: grid;
  }
}

@keyframes column-in {
  from {
    opacity: 0;
    transform: translateX(10px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .conversation,
  .avatar,
  .thread-link,
  .back-button,
  .list-fade-enter-active,
  .list-fade-leave-active {
    transition-duration: 1ms;
  }

  .unread-badge,
  .inbox,
  .thread {
    animation: none;
  }
}
</style>
