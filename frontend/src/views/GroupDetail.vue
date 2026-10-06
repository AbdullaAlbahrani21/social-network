<script setup>
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'
import { useChatStore } from '../stores/chat'
import { useWebSocketStore } from '../stores/websocket'
import ChatWindow from '../components/ChatWindow.vue'
import ConfirmDelete from '../components/ConfirmDelete.vue'
import EmojiButton from '../components/EmojiButton.vue'
import NavIcon from '../components/NavIcon.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'

const auth = useAuthStore()
const chatStore = useChatStore()
const ws = useWebSocketStore()
const route = useRoute()
const router = useRouter()

const groupId = computed(() => Number(route.params.id))

const group = ref(null)
const members = ref([])
const pendingRequests = ref([])
const posts = ref([])
const events = ref([])
const loading = ref(false)
const notFound = ref(false)

const error = ref(null)
const membersError = ref(null)
const joinRequestsError = ref(null)
const postsError = ref(null)
const eventsError = ref(null)

// api.js keeps only body.message and throws the status away, so matching the text is the only way to tell "no such group" from a real failure.
const notFoundMessage = 'group not found'

const isMember = computed(() => !!group.value?.isMember)
const isCreator = computed(() => group.value?.role === 'creator')

const joinRequest = reactive({ sending: false, sent: false, error: null })

const requestActions = reactive({})
const eventActions = reactive({})
const idleAction = { working: false, error: null }

const inviteQuery = ref('')
const inviteResults = ref([])
const inviteSearchedQuery = ref('')
const inviteSearching = ref(false)
const inviteConfirmation = ref(null)
const inviteSearchError = ref(null)
const inviteActions = reactive({})

const composeKind = ref(null)

const newPostContent = ref('')
const postSubmitting = ref(false)
const newPostError = ref(null)

const newEventTitle = ref('')
const newEventDescription = ref('')
const newEventTime = ref('')
const newEventMin = ref('')
const newEventMax = ref('')

// Mirrors maxEventYearsAhead in backend/pkg/events/models.go, which is what actually enforces it.
const MAX_EVENT_YEARS_AHEAD = 2
const eventSubmitting = ref(false)
const newEventError = ref(null)

const deleting = ref(false)
const deleteError = ref(null)

const membersDialog = ref(null)
const requestsDialog = ref(null)

const maxPostLength = 5000

// Spread rather than .length: the server counts runes, JS string length counts UTF-16 code units.
const postLength = computed(() => [...newPostContent.value].length)

function actionState(invitationId) {
  return requestActions[invitationId] || idleAction
}

function eventActionState(eventId) {
  return eventActions[eventId] || idleAction
}

function inviteActionState(userId) {
  return inviteActions[userId] || idleAction
}

function clearKeyed(map) {
  for (const key of Object.keys(map)) {
    delete map[key]
  }
}

function openMembers() {
  membersDialog.value?.showModal()
}

function closeMembers() {
  membersDialog.value?.close()
}

function openRequests() {
  requestsDialog.value?.showModal()
}

function closeRequests() {
  requestsDialog.value?.close()
}

// A click that lands on the <dialog> itself, not the panel inside it, was on the backdrop.
function onDialogClick(event, dialog) {
  if (event.target === dialog) dialog.close()
}

let loadToken = 0

async function loadGroup(token) {
  try {
    const result = await api.get(`/api/groups/${groupId.value}`)
    if (token !== loadToken) return
    group.value = result
  } catch (e) {
    if (token !== loadToken) return
    if (e.message === notFoundMessage) notFound.value = true
    else error.value = e.message
  }
}

async function loadMembers(token) {
  try {
    const result = await api.get(`/api/groups/${groupId.value}/members`)
    if (token !== loadToken) return
    members.value = result
  } catch (e) {
    if (token !== loadToken) return
    membersError.value = e.message
  }
}

async function loadJoinRequests(token) {
  try {
    const result = await api.get(`/api/groups/${groupId.value}/join-requests`)
    if (token !== loadToken) return
    pendingRequests.value = result
  } catch (e) {
    if (token !== loadToken) return
    joinRequestsError.value = e.message
  }
}

async function loadPosts(token) {
  try {
    const result = await api.get(`/api/groups/${groupId.value}/posts`)
    if (token !== loadToken) return
    posts.value = result
  } catch (e) {
    if (token !== loadToken) return
    postsError.value = e.message
  }
}

async function loadEvents(token) {
  try {
    const result = await api.get(`/api/groups/${groupId.value}/events`)
    if (token !== loadToken) return
    events.value = result
  } catch (e) {
    if (token !== loadToken) return
    eventsError.value = e.message
  }
}

// In sequence, not parallel: /members, /posts, /events and the chat all 403 for a non-member, and isMember only arrives with the group itself.
async function load() {
  const token = ++loadToken

  loading.value = true
  notFound.value = false
  error.value = null
  membersError.value = null
  joinRequestsError.value = null
  postsError.value = null
  eventsError.value = null
  group.value = null
  members.value = []
  pendingRequests.value = []
  posts.value = []
  events.value = []
  clearKeyed(requestActions)
  clearKeyed(eventActions)
  resetInviteSearch()

  composeKind.value = null
  newPostContent.value = ''
  newPostError.value = null
  newEventTitle.value = ''
  newEventDescription.value = ''
  newEventTime.value = ''
  newEventError.value = null
  deleteError.value = null

  try {
    await loadGroup(token)
    if (token !== loadToken) return

    if (group.value?.isMember) {
      await loadMembers(token)
      if (token !== loadToken) return
    }

    if (group.value?.role === 'creator') {
      await loadJoinRequests(token)
      if (token !== loadToken) return
    }

    if (group.value?.isMember) {
      await loadPosts(token)
      if (token !== loadToken) return

      await loadEvents(token)
      if (token !== loadToken) return

      chatStore.setActiveChat(groupId.value, 'group')
    }
  } finally {
    if (token === loadToken) loading.value = false
  }
}

async function requestToJoin() {
  joinRequest.sending = true
  joinRequest.sent = false
  joinRequest.error = null

  try {
    await api.post(`/api/groups/${groupId.value}/join-request`, {})
    joinRequest.sending = false
    joinRequest.sent = true
  } catch (e) {
    joinRequest.sending = false
    joinRequest.error = e.message
  }
}

function dropRequest(invitationId) {
  pendingRequests.value = pendingRequests.value.filter(
    (request) => request.invitationId !== invitationId,
  )

  delete requestActions[invitationId]
}

async function refreshAfterAccept() {
  const token = loadToken

  try {
    const result = await api.get(`/api/groups/${groupId.value}`)
    if (token !== loadToken) return
    group.value = result
  } catch {
    return
  }

  await loadMembers(token)
}

async function resolveRequest(request, action) {
  requestActions[request.invitationId] = { working: true, error: null }

  try {
    await api.post(`/api/group-invitations/${request.invitationId}/${action}`, {})
  } catch (e) {
    requestActions[request.invitationId] = { working: false, error: e.message }
    return
  }

  dropRequest(request.invitationId)

  if (action === 'accept') await refreshAfterAccept()

  if (pendingRequests.value.length === 0) closeRequests()
}

// Separate from loadToken: that one answers "is this still the group on screen", this one "is this still the query in the box".
let inviteSearchToken = 0
let inviteDebounceTimer = null

function resetInviteSearch() {
  clearTimeout(inviteDebounceTimer)
  inviteSearchToken += 1

  inviteQuery.value = ''
  inviteResults.value = []
  inviteSearchedQuery.value = ''
  inviteSearching.value = false
  inviteSearchError.value = null
  inviteConfirmation.value = null
  clearKeyed(inviteActions)
}

async function searchInvitableUsers() {
  const search = inviteQuery.value.trim()
  const token = ++inviteSearchToken

  // The endpoint answers a blank query with a 400 by design, so one is never sent.
  if (!search) {
    inviteResults.value = []
    inviteSearchedQuery.value = ''
    inviteSearchError.value = null
    inviteSearching.value = false
    return
  }

  inviteSearching.value = true
  inviteSearchError.value = null
  inviteConfirmation.value = null

  try {
    const result = await api.get(
      `/api/groups/${groupId.value}/invitable-users?search=${encodeURIComponent(search)}`,
    )
    if (token !== inviteSearchToken) return
    inviteResults.value = result
    inviteSearchedQuery.value = search
  } catch (e) {
    if (token !== inviteSearchToken) return
    inviteSearchError.value = e.message
    inviteResults.value = []
    inviteSearchedQuery.value = search
  } finally {
    if (token === inviteSearchToken) inviteSearching.value = false
  }
}

watch(inviteQuery, () => {
  clearTimeout(inviteDebounceTimer)
  inviteDebounceTimer = setTimeout(searchInvitableUsers, 300)
})

onUnmounted(() => clearTimeout(inviteDebounceTimer))

function dropInvitableUser(userId) {
  inviteResults.value = inviteResults.value.filter((user) => user.userId !== userId)
  delete inviteActions[userId]
}

async function inviteUser(user) {
  inviteActions[user.userId] = { working: true, error: null }
  inviteConfirmation.value = null

  try {
    await api.post(`/api/groups/${groupId.value}/invite`, { userId: user.userId })
  } catch (e) {
    inviteActions[user.userId] = { working: false, error: e.message }
    return
  }

  dropInvitableUser(user.userId)
  inviteConfirmation.value = `Invitation sent to ${personLabel(user)}.`
}

function toggleCompose(kind) {
  composeKind.value = composeKind.value === kind ? null : kind

  if (composeKind.value === 'event') {
    const now = new Date()
    const latest = new Date(now)
    latest.setFullYear(latest.getFullYear() + MAX_EVENT_YEARS_AHEAD)

    newEventMin.value = localDateTimeForInput(now)
    newEventMax.value = localDateTimeForInput(latest)
  }
}

// Built from the local parts rather than toISOString, which would hand over UTC and shift the bounds by the timezone offset.
function localDateTimeForInput(date) {
  const pad = (n) => String(n).padStart(2, '0')

  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}`
  )
}

async function createPost() {
  const content = newPostContent.value.trim()

  if (!content) {
    newPostError.value = 'Write something before posting.'
    return
  }

  postSubmitting.value = true
  newPostError.value = null

  try {
    const created = await api.post(`/api/groups/${groupId.value}/posts`, { content })

    postsError.value = null
    posts.value = [created, ...posts.value]
    newPostContent.value = ''
    composeKind.value = null
  } catch (e) {
    newPostError.value = e.message
  } finally {
    postSubmitting.value = false
  }
}

// An event list is ordered by when the event happens, not when it was created, so a new one is inserted rather than prepended.
// The creator's own tab inserts from the POST reply and then sees the broadcast for the same event, so an id already here wins.
function insertEvent(created) {
  if (events.value.some((existing) => existing.id === created.id)) return

  const at = Date.parse(created.eventTime)
  const next = [...events.value]
  const index = next.findIndex((existing) => Date.parse(existing.eventTime) > at)

  if (index === -1) next.push(created)
  else next.splice(index, 0, created)

  events.value = next
}

// Live events for whichever group is on screen: the socket outlives this view, so the subscription has to be dropped with it.
const stopEventFeed = ws.on('group_event', (data) => {
  if (!data.event || Number(data.groupId) !== groupId.value) return
  if (!isMember.value) return

  insertEvent(data.event)
})

onUnmounted(stopEventFeed)

async function createEvent() {
  const title = newEventTitle.value.trim()
  const description = newEventDescription.value.trim()

  if (!title) {
    newEventError.value = 'Give the event a title.'
    return
  }

  if (!newEventTime.value) {
    newEventError.value = 'Choose when the event happens.'
    return
  }

  // A datetime-local value carries no offset and is read as local time, so toISOString gives the RFC3339 UTC instant the API wants.
  const when = new Date(newEventTime.value)

  if (Number.isNaN(when.getTime())) {
    newEventError.value = 'That date and time could not be read.'
    return
  }

  eventSubmitting.value = true
  newEventError.value = null

  try {
    const created = await api.post(`/api/groups/${groupId.value}/events`, {
      title,
      description,
      eventTime: when.toISOString(),
    })

    eventsError.value = null
    insertEvent(created)

    newEventTitle.value = ''
    newEventDescription.value = ''
    newEventTime.value = ''
    composeKind.value = null
  } catch (e) {
    newEventError.value = e.message
  } finally {
    eventSubmitting.value = false
  }
}

// Whatever bucket the viewer was in loses one and the new one gains; a null previous response means no bucket, and the floor at zero is defensive.
function applyRsvp(event, previous, next) {
  if (previous === next) return

  if (previous === 'going') event.goingCount = Math.max(0, event.goingCount - 1)
  else if (previous === 'not_going') event.notGoingCount = Math.max(0, event.notGoingCount - 1)

  if (next === 'going') event.goingCount += 1
  else if (next === 'not_going') event.notGoingCount += 1

  event.viewerResponse = next
}

async function rsvp(event, response) {
  if (event.viewerResponse === response) return

  eventActions[event.id] = { working: true, error: null }

  try {
    const result = await api.post(`/api/events/${event.id}/rsvp`, { response })

    applyRsvp(event, event.viewerResponse, result.response)
    delete eventActions[event.id]
  } catch (e) {
    eventActions[event.id] = { working: false, error: e.message }
  }
}

async function deleteGroup() {
  deleting.value = true
  deleteError.value = null

  try {
    await api.del(`/api/groups/${groupId.value}`)
  } catch (e) {
    deleting.value = false
    deleteError.value = e.message
    return
  }

  router.push('/groups')
}

function personLabel(person) {
  return person.nickname ? `${person.name} (${person.nickname})` : person.name
}

function authorLabel(post) {
  return personLabel({ name: post.authorName, nickname: post.authorNickname })
}

const memberNames = computed(() => {
  const names = {}
  for (const member of members.value) {
    names[member.userId] = member.nickname || member.name
  }
  return names
})

function nameOf(userId) {
  return memberNames.value[userId] || null
}

// The first ten characters as given: round-tripping through Date would move the day into the reader's timezone.
function dateOnly(value) {
  return (value || '').slice(0, 10)
}

function eventWhen(value) {
  const at = new Date(value)
  if (Number.isNaN(at.getTime())) return value
  return at.toLocaleString()
}

function responseLabel(response) {
  if (response === 'going') return 'Going'
  if (response === 'not_going') return 'Not going'
  return 'No response yet'
}

const createdOn = computed(() => dateOnly(group.value?.createdAt))

watch(
  () => [auth.isLoggedIn, route.params.id],
  ([loggedIn]) => {
    if (loggedIn) load()
  },
  { immediate: true },
)

const eventsOpen = ref(true)
const postsOpen = ref(true)

const rsvpOptions = [
  { value: 'going', label: 'Going' },
  { value: 'not_going', label: 'Not going' },
]

function initialOf(post) {
  const source = post.authorNickname || post.authorName || ''
  return [...source][0]?.toUpperCase() || '·'
}

const TONES = ['tone-purple', 'tone-pink', 'tone-muted']

function toneFor(index) {
  return TONES[index % TONES.length]
}

function openPost(post, event) {
  if (event.target.closest("a, button, input, textarea, select")) return
  if (window.getSelection()?.toString()) return
  router.push(`/posts/${post.id}`)
}
</script>

<template>
  <main class="group-detail">
    <p v-if="auth.loading && !auth.isLoggedIn" class="status">Checking your session...</p>

    <p v-else-if="!auth.isLoggedIn" class="status">
      You need to be logged in to view this group.
      <RouterLink to="/login">Log in</RouterLink>
    </p>

    <template v-else>
      <p class="back">
        <RouterLink to="/groups" class="back-link">
          <NavIcon name="back" />
          <span>Back to groups</span>
        </RouterLink>
      </p>

      <p v-if="loading" class="status loading-status">Loading group...</p>

      <p v-else-if="notFound" class="status">Group not found.</p>

      <p v-else-if="error" class="error" role="alert">{{ error }}</p>

      <template v-else-if="group">
        <header class="group-head">
          <span class="group-badge" aria-hidden="true"><NavIcon name="groups" /></span>

          <div class="head-text">
            <h1 class="group-title">{{ group.title }}</h1>
            <p class="group-description">{{ group.description }}</p>
            <p class="group-meta">
              Created by
              <RouterLink :to="`/profile/${group.creatorId}`">{{ group.creatorName }}</RouterLink>
              on {{ createdOn }}
            </p>
          </div>

          <div class="head-actions">
            <button
              v-if="isMember"
              type="button"
              class="members-pill"
              aria-haspopup="dialog"
              @click="openMembers"
            >
              Members ({{ group.memberCount }})
            </button>

            <div v-if="isCreator" class="creator-tools">
              <span class="tools-label">Creator tools</span>

              <div class="tools-row">
                <button
                  type="button"
                  class="tool-button"
                  aria-haspopup="dialog"
                  @click="openRequests"
                >
                  <NavIcon name="mail" />
                  <span>Join requests</span>
                  <span v-if="pendingRequests.length" class="pill">
                    {{ pendingRequests.length }}
                  </span>
                </button>

                <ConfirmDelete noun="group" @confirm="deleteGroup" />
              </div>

              <span v-if="deleting" class="status tools-status">Deleting...</span>
            </div>

            <p v-if="deleteError" class="error" role="alert">{{ deleteError }}</p>
          </div>
        </header>

        <section v-if="!isMember" class="stranger">
          <span class="stranger-icon" aria-hidden="true"><NavIcon name="lock" /></span>

          <p class="status stranger-text">
            Only members can see this group's members, posts, events and chat.
          </p>

          <button
            type="button"
            class="join-pill"
            :class="{ 'is-sent': joinRequest.sent }"
            :disabled="joinRequest.sending || joinRequest.sent"
            @click="requestToJoin"
          >
            {{
              joinRequest.sent
                ? 'Request sent'
                : joinRequest.sending
                  ? 'Sending...'
                  : 'Request to join'
            }}
          </button>

          <p v-if="joinRequest.error" class="error" role="alert">{{ joinRequest.error }}</p>
        </section>

        <div v-else class="group-body">
          <section class="chat-panel" aria-labelledby="chat-title">
            <h2 id="chat-title" class="chat-title">Group chat</h2>

            <div class="chat-frame">
              <ChatWindow :name-of="nameOf">
                <template #compose-lead>
                  <div class="compose-anchor">
                    <button
                      type="button"
                      class="plus-button"
                      :class="{ 'is-open': composeKind !== null }"
                      :aria-expanded="composeKind !== null"
                      aria-label="Add a post or an event"
                      @click="composeKind = composeKind ? null : 'menu'"
                    >
                      <NavIcon name="plus" />
                    </button>

                    <Transition name="pop">
                      <div v-if="composeKind" class="compose-pop">
                        <div class="pop-choices">
                          <button
                            type="button"
                            class="pop-choice"
                            :class="{ 'is-active': composeKind === 'post' }"
                            @click="toggleCompose('post')"
                          >
                            <NavIcon name="text" />
                            <span>Add post</span>
                          </button>

                          <button
                            type="button"
                            class="pop-choice"
                            :class="{ 'is-active': composeKind === 'event' }"
                            @click="toggleCompose('event')"
                          >
                            <NavIcon name="calendar" />
                            <span>Add event</span>
                          </button>
                        </div>

                        <form
                          v-if="composeKind === 'post'"
                          class="compose-form"
                          @submit.prevent="createPost"
                        >
                          <label for="new-post" class="field-label">Write a post</label>
                          <textarea
                            id="new-post"
                            v-model="newPostContent"
                            class="text-input"
                            rows="4"
                            :disabled="postSubmitting"
                          ></textarea>
                          <EmojiButton target="new-post" />

                          <p class="status count">{{ postLength }} / {{ maxPostLength }}</p>

                          <button type="submit" class="submit-pill" :disabled="postSubmitting">
                            {{ postSubmitting ? 'Posting...' : 'Post' }}
                          </button>

                          <p v-if="newPostError" class="error" role="alert">{{ newPostError }}</p>
                        </form>

                        <form
                          v-else-if="composeKind === 'event'"
                          class="compose-form"
                          @submit.prevent="createEvent"
                        >
                          <label for="new-event-title" class="field-label">Event title</label>
                          <input
                            id="new-event-title"
                            v-model="newEventTitle"
                            type="text"
                            class="text-input"
                            :disabled="eventSubmitting"
                          />
                          <EmojiButton target="new-event-title" />

                          <label for="new-event-description" class="field-label">
                            Description (optional)
                          </label>
                          <textarea
                            id="new-event-description"
                            v-model="newEventDescription"
                            class="text-input"
                            rows="3"
                            :disabled="eventSubmitting"
                          ></textarea>
                          <EmojiButton target="new-event-description" />

                          <label for="new-event-time" class="field-label">When</label>
                          <input
                            id="new-event-time"
                            v-model="newEventTime"
                            type="datetime-local"
                            class="text-input"
                            :min="newEventMin"
                            :max="newEventMax"
                            :disabled="eventSubmitting"
                          />

                          <button type="submit" class="submit-pill" :disabled="eventSubmitting">
                            {{ eventSubmitting ? 'Creating...' : 'Create event' }}
                          </button>

                          <p v-if="newEventError" class="error" role="alert">
                            {{ newEventError }}
                          </p>
                        </form>
                      </div>
                    </Transition>
                  </div>
                </template>
              </ChatWindow>
            </div>
          </section>

          <div class="secondary">
            <section class="panel" aria-labelledby="events-title">
              <button
                type="button"
                class="panel-head"
                :aria-expanded="eventsOpen"
                aria-controls="events-body"
                @click="eventsOpen = !eventsOpen"
              >
                <span class="panel-icon" aria-hidden="true"><NavIcon name="calendar" /></span>
                <h2 id="events-title" class="panel-title">Events</h2>
                <span v-if="events.length" class="panel-count">{{ events.length }}</span>
                <NavIcon name="back" class="panel-chevron" :class="{ 'is-open': eventsOpen }" />
              </button>

              <Transition name="collapse">
                <div v-show="eventsOpen" id="events-body" class="panel-body">
                  <p v-if="eventsError" class="error" role="alert">{{ eventsError }}</p>

                  <p v-else-if="events.length === 0" class="status empty">
                    No events in this group yet.
                  </p>

                  <ul v-else class="event-list">
                    <li v-for="event in events" :key="event.id" class="event-card">
                      <div class="event-head">
                        <span class="event-date" aria-hidden="true">
                          <NavIcon name="calendar" />
                        </span>
                        <div class="event-headings">
                          <h3 class="event-title">{{ event.title }}</h3>
                          <p class="meta">
                            {{ eventWhen(event.eventTime) }} &middot; organised by
                            <RouterLink :to="`/profile/${event.creatorId}`">
                              {{ event.creatorName }}
                            </RouterLink>
                          </p>
                        </div>
                      </div>

                      <p v-if="event.description" class="event-description">
                        {{ event.description }}
                      </p>

                      <div class="rsvp">
                        <SegmentedToggle
                          class="rsvp-toggle"
                          :options="rsvpOptions"
                          :model-value="event.viewerResponse"
                          label="Your response"
                          appearance="outline"
                          :disabled="eventActionState(event.id).working"
                          @select="rsvp(event, $event)"
                        />

                        <p class="rsvp-counts">
                          <span class="count-chip">{{ event.goingCount }} going</span>
                          <span class="count-chip">{{ event.notGoingCount }} not going</span>
                          <span class="you">You: {{ responseLabel(event.viewerResponse) }}</span>
                        </p>
                      </div>

                      <p v-if="eventActionState(event.id).error" class="error" role="alert">
                        {{ eventActionState(event.id).error }}
                      </p>
                    </li>
                  </ul>
                </div>
              </Transition>
            </section>

            <section class="panel" aria-labelledby="posts-title">
              <button
                type="button"
                class="panel-head"
                :aria-expanded="postsOpen"
                aria-controls="posts-body"
                @click="postsOpen = !postsOpen"
              >
                <span class="panel-icon" aria-hidden="true"><NavIcon name="text" /></span>
                <h2 id="posts-title" class="panel-title">Posts</h2>
                <span v-if="posts.length" class="panel-count">{{ posts.length }}</span>
                <NavIcon name="back" class="panel-chevron" :class="{ 'is-open': postsOpen }" />
              </button>

              <Transition name="collapse">
                <div v-show="postsOpen" id="posts-body" class="panel-body">
                  <p v-if="postsError" class="error" role="alert">{{ postsError }}</p>

                  <p v-else-if="posts.length === 0" class="status empty">
                    No posts in this group yet.
                  </p>

                  <ul v-else class="post-list">
                    <li
                      v-for="(post, index) in posts"
                      :key="post.id"
                      class="post-card"
                      @click="openPost(post, $event)"
                    >
                      <div class="post-head">
                        <span class="post-avatar" :class="toneFor(index)" aria-hidden="true">
                          {{ initialOf(post) }}
                        </span>
                        <p class="meta post-byline">
                          <RouterLink :to="`/profile/${post.userId}`" class="post-author">
                            {{ authorLabel(post) }}
                          </RouterLink>
                          <span class="post-date">{{ dateOnly(post.createdAt) }}</span>
                        </p>
                      </div>

                      <!-- Interpolated, never v-html: post content is whatever another member typed. -->
                      <p class="post-content">{{ post.content }}</p>

                      <footer class="post-actions">
                        <RouterLink :to="`/posts/${post.id}`" class="post-action">
                          <NavIcon name="messages" />
                          <span>Comments</span>
                        </RouterLink>
                      </footer>
                    </li>
                  </ul>
                </div>
              </Transition>
            </section>
          </div>
        </div>

        <dialog
          v-if="isMember"
          ref="membersDialog"
          class="slide-over"
          aria-labelledby="members-dialog-title"
          @click="onDialogClick($event, membersDialog)"
        >
          <div class="dialog-panel">
            <span class="panel-accent" aria-hidden="true"></span>

            <header class="dialog-head">
              <h2 id="members-dialog-title" class="dialog-title">
                Members ({{ group.memberCount }})
              </h2>
              <button type="button" class="icon-button" aria-label="Close" @click="closeMembers">
                <NavIcon name="close" />
              </button>
            </header>

            <div class="dialog-body">
              <p v-if="membersError" class="error" role="alert">{{ membersError }}</p>

              <ul v-else class="people">
                <li v-for="(member, index) in members" :key="member.userId">
                  <RouterLink
                    :to="`/profile/${member.userId}`"
                    class="person"
                    @click="closeMembers"
                  >
                    <span class="person-avatar" :class="toneFor(index)" aria-hidden="true">
                      {{ (member.nickname || member.name || '·').charAt(0).toUpperCase() }}
                    </span>
                    <span class="person-name">{{ personLabel(member) }}</span>
                    <span class="person-tag" :class="{ 'is-creator': member.role === 'creator' }">
                      {{ member.role === 'creator' ? 'creator' : 'member' }}
                    </span>
                  </RouterLink>
                </li>
              </ul>

              <section class="invite" aria-labelledby="invite-title">
                <h3 id="invite-title" class="invite-title">Invite people</h3>

                <form class="invite-search" @submit.prevent="searchInvitableUsers">
                  <label for="invite-search" class="visually-hidden">Search people</label>
                  <NavIcon name="search" class="search-icon" />
                  <input
                    id="invite-search"
                    v-model="inviteQuery"
                    type="search"
                    maxlength="100"
                    class="search-input"
                    placeholder="Search by name, nickname or email"
                  />
                </form>

                <Transition name="fade">
                  <p v-if="inviteConfirmation" class="invite-confirmation">
                    <NavIcon name="mail" />
                    <span>{{ inviteConfirmation }}</span>
                  </p>
                </Transition>

                <p v-if="inviteSearchError" class="error" role="alert">{{ inviteSearchError }}</p>

                <p v-else-if="inviteSearching" class="status">Searching...</p>

                <p v-else-if="!inviteSearchedQuery" class="status">
                  Search for someone to invite.
                </p>

                <p v-else-if="inviteResults.length === 0" class="status">
                  No matching users found.
                </p>

                <TransitionGroup v-else tag="ul" name="invite-row" class="people">
                  <li v-for="user in inviteResults" :key="user.userId" class="invite-row">
                    <span class="person-avatar tone-muted" aria-hidden="true">
                      {{ (user.nickname || user.name || '·').charAt(0).toUpperCase() }}
                    </span>

                    <span class="person-name">{{ personLabel(user) }}</span>

                    <button
                      type="button"
                      class="invite-pill"
                      :disabled="inviteActionState(user.userId).working"
                      @click="inviteUser(user)"
                    >
                      {{ inviteActionState(user.userId).working ? 'Inviting...' : 'Invite' }}
                    </button>

                    <p v-if="inviteActionState(user.userId).error" class="error" role="alert">
                      {{ inviteActionState(user.userId).error }}
                    </p>
                  </li>
                </TransitionGroup>
              </section>
            </div>
          </div>
        </dialog>

        <dialog
          v-if="isCreator"
          ref="requestsDialog"
          class="slide-over"
          aria-labelledby="requests-dialog-title"
          @click="onDialogClick($event, requestsDialog)"
        >
          <div class="dialog-panel">
            <span class="panel-accent" aria-hidden="true"></span>

            <header class="dialog-head">
              <h2 id="requests-dialog-title" class="dialog-title">Join requests</h2>
              <button type="button" class="icon-button" aria-label="Close" @click="closeRequests">
                <NavIcon name="close" />
              </button>
            </header>

            <div class="dialog-body">
              <p v-if="joinRequestsError" class="error" role="alert">{{ joinRequestsError }}</p>

              <p v-else-if="pendingRequests.length === 0" class="status">
                No pending join requests.
              </p>

              <TransitionGroup v-else tag="ul" name="request-row" class="request-list">
                <li
                  v-for="(request, index) in pendingRequests"
                  :key="request.invitationId"
                  class="request-row"
                >
                  <span class="person-avatar" :class="toneFor(index)" aria-hidden="true">
                    {{ (request.nickname || request.name || '·').charAt(0).toUpperCase() }}
                  </span>

                  <div class="request-text">
                    <RouterLink :to="`/profile/${request.userId}`" class="person-name">
                      {{ personLabel(request) }}
                    </RouterLink>
                    <span class="meta">requested {{ dateOnly(request.createdAt) }}</span>
                  </div>

                  <div class="request-actions">
                    <button
                      type="button"
                      class="pill-ghost"
                      :disabled="actionState(request.invitationId).working"
                      @click="resolveRequest(request, 'decline')"
                    >
                      Decline
                    </button>

                    <button
                      type="button"
                      class="pill-solid"
                      :disabled="actionState(request.invitationId).working"
                      @click="resolveRequest(request, 'accept')"
                    >
                      Accept
                    </button>
                  </div>

                  <p v-if="actionState(request.invitationId).error" class="error" role="alert">
                    {{ actionState(request.invitationId).error }}
                  </p>
                </li>
              </TransitionGroup>
            </div>
          </div>
        </dialog>
      </template>
    </template>
  </main>
</template>

<style scoped src="../styles/controls.css"></style>

<style scoped>
.group-detail {
  display: flex;
  flex-direction: column;
  max-width: 64rem;
  min-height: calc(100vh - 2 * 40px);
  margin: 0 auto;
  padding-block: 28px var(--space-6);
}

.back {
  margin: 0 0 var(--space-3);
}

.back-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
  font-size: var(--text-meta);
  font-weight: 700;
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out);
}

.back-link:hover {
  color: var(--primary);
}

.back-link :deep(.nav-icon) {
  width: 1rem;
  height: 1rem;
}

.loading-status {
  animation: appear-late 1ms 300ms both;
}

@keyframes appear-late {
  from { opacity: 0; }
  to { opacity: 1; }
}

.group-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 18px;
  margin-bottom: 18px;
  padding: 22px 24px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
}

.group-badge {
  display: grid;
  flex: none;
  place-items: center;
  width: 3.5rem;
  height: 3.5rem;
  background: var(--panel-gradient);
  border-radius: var(--radius-md);
  box-shadow: 0 10px 22px -12px color-mix(in srgb, var(--primary) 70%, transparent);
  color: var(--surface);
}

.group-badge :deep(.nav-icon) {
  width: 1.6rem;
  height: 1.6rem;
}

.head-text {
  flex: 1 1 18rem;
  min-width: 0;
}

.group-title {
  margin: 0;
  font-size: 1.75rem;
  font-weight: 800;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.group-description {
  margin: 6px 0 0;
  color: var(--text-muted);
  font-size: var(--text-body);
  overflow-wrap: anywhere;
}

.group-meta {
  margin: 10px 0 0;
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.head-actions {
  display: flex;
  flex: none;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
  min-width: 0;
}

.members-pill {
  min-height: 2.25rem;
  padding: 0 18px;
  background-color: color-mix(in srgb, var(--text) 6%, var(--surface));
  border: 0;
  border-radius: var(--radius-pill);
  color: var(--text-muted);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.members-pill:hover {
  background-color: color-mix(in srgb, var(--text) 10%, var(--surface));
  color: var(--text);
  transform: translateY(-1px);
}

.members-pill:active {
  transform: translateY(0);
  transition-duration: 80ms;
}

.members-pill:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.creator-tools {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  padding: 10px 12px;
  background-color: var(--bg);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
}

.tools-label {
  color: var(--text-placeholder);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.tools-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}

.tool-button {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 2.125rem;
  padding: 0 14px;
  background-color: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-pill);
  color: var(--text);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.tool-button:hover {
  border-color: var(--border-strong);
  box-shadow: 0 8px 18px -12px color-mix(in srgb, var(--primary) 50%, transparent);
  transform: translateY(-1px);
}

.tool-button:active {
  transform: translateY(0);
  transition-duration: 80ms;
}

.tool-button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.tool-button :deep(.nav-icon) {
  width: 1rem;
  height: 1rem;
  color: var(--text-muted);
}

.pill {
  min-width: 1.25rem;
  padding: 0 6px;
  background-color: var(--notification-dot);
  border-radius: var(--radius-pill);
  color: var(--text);
  font-size: 0.7rem;
  font-weight: 800;
  line-height: 1.25rem;
  text-align: center;
}

.tools-status {
  font-size: var(--text-meta);
}

.creator-tools :deep(.confirm-delete) {
  margin-left: 0;
}

.stranger {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 40px var(--space-5);
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  text-align: center;
}

.stranger-icon {
  display: grid;
  place-items: center;
  width: 3rem;
  height: 3rem;
  background-color: var(--badge-purple-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-purple-fg);
}

.stranger-text {
  max-width: 28rem;
  margin: 0;
}

.join-pill {
  min-height: 2.75rem;
  padding: 0 26px;
  background-color: var(--primary);
  border: 1px solid transparent;
  border-radius: var(--radius-pill);
  box-shadow: var(--shadow-primary);
  color: var(--surface);
  font-size: var(--text-sm);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-base) var(--ease-out),
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    color var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.join-pill:hover:not(:disabled) {
  background-color: var(--primary-hover);
  box-shadow: var(--shadow-primary-lift);
  transform: translateY(-2px);
}

.join-pill:active:not(:disabled) {
  transform: translateY(0);
  transition-duration: 80ms;
}

.join-pill:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 3px;
}

.join-pill.is-sent {
  background-color: var(--badge-muted-bg);
  box-shadow: none;
  color: var(--badge-muted-fg);
}

.join-pill:disabled {
  cursor: default;
}

.group-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 18px;
  min-height: 0;
}

.chat-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.chat-title {
  margin: 0 0 10px;
  font-size: var(--text-h2);
  font-weight: 800;
}

.chat-frame {
  display: flex;
  height: clamp(26rem, 58vh, 40rem);
  overflow: hidden;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
}

.chat-frame > :deep(.chat-window) {
  flex: 1;
  min-width: 0;
}

.compose-anchor {
  position: relative;
  flex: none;
  display: flex;
}

.plus-button {
  display: grid;
  place-items: center;
  width: 2.5rem;
  height: 2.5rem;
  background-color: color-mix(in srgb, var(--primary) 10%, var(--surface));
  border: 0;
  border-radius: var(--radius-pill);
  color: var(--primary);
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.plus-button:hover {
  background-color: var(--primary);
  box-shadow: var(--shadow-primary);
  color: var(--surface);
  transform: translateY(-1px);
}

.plus-button:active {
  transform: translateY(0);
  transition-duration: 80ms;
}

.plus-button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.plus-button.is-open {
  background-color: var(--primary);
  box-shadow: var(--shadow-primary);
  color: var(--surface);
}

.plus-button :deep(.nav-icon) {
  width: 1.2rem;
  height: 1.2rem;
  transition: transform var(--duration-base) var(--ease-out);
}

.plus-button.is-open :deep(.nav-icon) {
  transform: rotate(45deg);
}

.compose-pop {
  position: absolute;
  bottom: calc(100% + 10px);
  left: 0;
  z-index: 20;
  width: min(23rem, calc(100vw - 2 * var(--space-4)));
  padding: 10px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: 0 22px 46px -18px color-mix(in srgb, var(--primary) 50%, transparent);
}

.pop-choices {
  display: flex;
  gap: 6px;
}

.pop-choice {
  display: inline-flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 2.25rem;
  padding: 0 12px;
  background-color: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-pill);
  color: var(--text-muted);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.pop-choice:hover {
  border-color: var(--border-strong);
  color: var(--text);
}

.pop-choice:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.pop-choice.is-active {
  background-color: var(--badge-purple-bg);
  border-color: transparent;
  color: var(--badge-purple-fg);
}

.pop-choice :deep(.nav-icon) {
  width: 1rem;
  height: 1rem;
}

.compose-form {
  display: grid;
  gap: 8px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-hairline);
}

.compose-form .text-input {
  border-radius: var(--radius-xs);
}

.field-label {
  font-size: var(--text-label);
  font-weight: 700;
}

.count {
  justify-self: end;
  font-size: var(--text-meta);
}

.submit-pill {
  justify-self: start;
  min-height: 2.25rem;
  padding: 0 20px;
  background-color: var(--primary);
  border: 0;
  border-radius: var(--radius-pill);
  box-shadow: var(--shadow-primary);
  color: var(--surface);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.submit-pill:hover:not(:disabled) {
  background-color: var(--primary-hover);
  box-shadow: var(--shadow-primary-lift);
  transform: translateY(-1px);
}

.submit-pill:active:not(:disabled) {
  transform: translateY(0);
  transition-duration: 80ms;
}

.submit-pill:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.submit-pill:disabled {
  opacity: 0.6;
  cursor: progress;
}

.pop-enter-active,
.pop-leave-active {
  transition:
    opacity var(--duration-fast) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
  transform-origin: left bottom;
}

.pop-enter-from,
.pop-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.96);
}

.secondary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 16px;
  align-items: start;
}

.panel {
  overflow: hidden;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
}

.panel-head {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 14px 18px;
  background: none;
  border: 0;
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-out);
}

.panel-head:hover {
  background-color: var(--bg);
}

.panel-head:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: -2px;
}

.panel-icon {
  display: grid;
  flex: none;
  place-items: center;
  width: 2rem;
  height: 2rem;
  background-color: var(--badge-purple-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-purple-fg);
}

.panel-icon :deep(.nav-icon) {
  width: 1rem;
  height: 1rem;
}

.panel-title {
  flex: 1;
  margin: 0;
  font-size: var(--text-card-title);
  font-weight: 700;
}

.panel-count {
  padding: 2px 9px;
  background-color: var(--badge-muted-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-muted-fg);
  font-size: var(--text-meta);
  font-weight: 700;
}

.panel-chevron {
  flex: none;
  width: 1rem;
  height: 1rem;
  color: var(--text-placeholder);
  transform: rotate(-90deg);
  transition: transform var(--duration-base) var(--ease-out);
}

.panel-chevron.is-open {
  transform: rotate(90deg);
}

.panel-body {
  padding: 0 18px 18px;
}

.empty {
  margin: 0;
  padding: 10px 0 4px;
  font-size: var(--text-meta);
}

.collapse-enter-active,
.collapse-leave-active {
  overflow: hidden;
  transition:
    opacity var(--duration-base) var(--ease-out),
    max-height 320ms var(--ease-out);
  max-height: 60rem;
}

.collapse-enter-from,
.collapse-leave-to {
  max-height: 0;
  opacity: 0;
}

.event-list,
.post-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.event-card,
.post-card {
  padding: 14px 16px;
  background-color: var(--bg);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
}

.event-head {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.event-date {
  display: grid;
  flex: none;
  place-items: center;
  width: 2.25rem;
  height: 2.25rem;
  background-color: var(--badge-pink-bg);
  border-radius: var(--radius-md);
  color: var(--badge-pink-fg);
}

.event-date :deep(.nav-icon) {
  width: 1.05rem;
  height: 1.05rem;
}

.event-headings {
  min-width: 0;
}

.event-title {
  margin: 0;
  font-size: var(--text-card-title);
  font-weight: 700;
  overflow-wrap: anywhere;
}

.event-description {
  margin: 10px 0 0;
  color: var(--text-muted);
  font-size: var(--text-meta);
  overflow-wrap: anywhere;
}

.meta {
  margin: 2px 0 0;
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.rsvp {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-top: 12px;
}

.rsvp-toggle {
  flex: none;
}

.rsvp-counts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin: 0;
  font-size: var(--text-meta);
}

.count-chip {
  padding: 2px 9px;
  background-color: var(--badge-muted-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-muted-fg);
  font-weight: 700;
}

.you {
  color: var(--text-muted);
}

.post-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.post-avatar,
.person-avatar {
  display: grid;
  flex: none;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border-radius: var(--radius-pill);
  font-size: var(--text-meta);
  font-weight: 800;
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

.post-byline {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 8px;
  margin: 0;
  min-width: 0;
}

.post-author {
  color: var(--text);
  font-weight: 700;
  text-decoration: none;
}

.post-author:hover {
  text-decoration: underline;
}

.post-date {
  color: var(--text-muted);
}

.post-content {
  margin: 10px 0 0;
  font-size: var(--text-body);
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.post-card {
  cursor: pointer;
}

.post-actions {
  display: flex;
  margin-top: 8px;
}

.post-action {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: -8px;
  padding: 6px 8px;
  border-radius: var(--radius-pill);
  color: var(--text-muted);
  font-size: var(--text-meta);
  font-weight: 700;
  text-decoration: none;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.post-action:hover {
  background-color: var(--badge-purple-bg);
  color: var(--primary);
}

.post-action :deep(.nav-icon) {
  width: 1rem;
  height: 1rem;
}

.slide-over {
  width: min(28rem, 100%);
  max-width: none;
  height: 100%;
  max-height: 100%;
  margin: 0 0 0 auto;
  padding: 0;
  overflow: hidden;
  background-color: var(--surface);
  border: 0;
  border-radius: var(--radius-lg) 0 0 var(--radius-lg);
  box-shadow: -30px 0 60px -30px color-mix(in srgb, var(--primary) 50%, transparent);
  color: var(--text);
  translate: 100% 0;
  opacity: 0;
  transition:
    translate var(--duration-base) var(--ease-out),
    opacity var(--duration-base) var(--ease-out),
    overlay var(--duration-base) allow-discrete,
    display var(--duration-base) allow-discrete;
}

.slide-over:focus-visible {
  outline: none;
}

.slide-over[open] {
  display: flex;
  flex-direction: column;
  translate: 0 0;
  opacity: 1;
}

@starting-style {
  .slide-over[open] {
    translate: 100% 0;
    opacity: 0;
  }
}

.slide-over::backdrop {
  background-color: transparent;
  backdrop-filter: blur(2px);
  transition:
    background-color var(--duration-base) var(--ease-out),
    overlay var(--duration-base) allow-discrete,
    display var(--duration-base) allow-discrete;
}

.slide-over[open]::backdrop {
  background-color: color-mix(in srgb, var(--text) 32%, transparent);
}

@starting-style {
  .slide-over[open]::backdrop {
    background-color: transparent;
  }
}

.dialog-panel {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}

.panel-accent {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 4px;
  background: var(--panel-gradient);
}

.dialog-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: 20px 16px 16px 26px;
  border-bottom: 1px solid var(--border-hairline);
}

.dialog-title {
  margin: 0;
  font-size: var(--text-h2);
  font-weight: 800;
}

.icon-button {
  display: grid;
  flex: none;
  place-items: center;
  width: 2.5rem;
  height: 2.5rem;
  background-color: color-mix(in srgb, var(--text) 6%, var(--surface));
  border: 0;
  border-radius: 50%;
  color: var(--text-muted);
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.icon-button:hover {
  background-color: color-mix(in srgb, var(--text) 10%, var(--surface));
  color: var(--text);
  transform: rotate(90deg);
}

.icon-button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.icon-button :deep(.nav-icon) {
  width: 1.1rem;
  height: 1.1rem;
}

.dialog-body {
  flex: 1;
  padding: var(--space-4) 16px var(--space-5) 26px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.people,
.request-list {
  position: relative;
  margin: 0;
  padding: 0;
  list-style: none;
}

.person {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  color: inherit;
  text-decoration: none;
}

.person:hover .person-name {
  color: var(--primary);
}

.person-name {
  flex: 1;
  min-width: 0;
  font-size: var(--text-label);
  font-weight: 700;
  transition: color var(--duration-fast) var(--ease-out);
}

.person-tag {
  flex: none;
  padding: 2px 9px;
  background-color: var(--badge-muted-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-muted-fg);
  font-size: 0.7rem;
  font-weight: 800;
}

.person-tag.is-creator {
  background-color: var(--badge-purple-bg);
  color: var(--badge-purple-fg);
}

.invite {
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px solid var(--border-hairline);
}

.invite-title {
  margin: 0 0 10px;
  font-size: var(--text-card-title);
  font-weight: 700;
}

.invite-search {
  position: relative;
  margin-bottom: 10px;
}

.search-icon {
  position: absolute;
  top: 50%;
  left: 16px;
  width: 1.05rem;
  height: 1.05rem;
  color: var(--text-placeholder);
  transform: translateY(-50%);
  pointer-events: none;
  transition: color var(--duration-base) var(--ease-out);
}

.invite-search:focus-within .search-icon {
  color: var(--primary);
}

.search-input {
  width: 100%;
  min-height: 2.75rem;
  padding: 0 18px 0 calc(16px + 1.05rem + 10px);
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-pill);
  box-shadow: 0 6px 18px -12px color-mix(in srgb, var(--primary) 30%, transparent);
  color: var(--text);
  font-size: var(--text-body);
  transition:
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out);
}

.search-input::placeholder {
  color: var(--text-placeholder);
}

.search-input:hover {
  border-color: var(--border);
}

.search-input:focus-visible {
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
  outline: none;
  box-shadow:
    0 0 0 4px color-mix(in srgb, var(--primary) 14%, transparent),
    0 6px 18px -12px color-mix(in srgb, var(--primary) 30%, transparent);
}

.invite-confirmation {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  padding: 8px 12px;
  background-color: color-mix(in srgb, var(--success) 10%, var(--surface));
  border-radius: var(--radius-sm);
  color: var(--success);
  font-size: var(--text-meta);
  font-weight: 700;
}

.invite-confirmation :deep(.nav-icon) {
  flex: none;
  width: 1rem;
  height: 1rem;
}

.invite-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
}

.invite-pill {
  flex: none;
  min-height: 2rem;
  padding: 0 14px;
  background-color: var(--surface);
  border: 1px solid var(--primary);
  border-radius: var(--radius-pill);
  color: var(--primary);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.invite-pill:hover:not(:disabled) {
  background-color: color-mix(in srgb, var(--primary) 10%, var(--surface));
  transform: translateY(-1px);
}

.invite-pill:active:not(:disabled) {
  transform: translateY(0);
  transition-duration: 80ms;
}

.invite-pill:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.invite-pill:disabled {
  opacity: 0.6;
  cursor: progress;
}

/* Doubled class so the -move rule below cannot override the transition and drop opacity from it. */
.invite-row.invite-row-leave-active,
.request-row.request-row-leave-active {
  position: absolute;
  right: 0;
  left: 0;
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.invite-row-leave-to,
.request-row-leave-to {
  opacity: 0;
  transform: translateX(16px);
}

.invite-row-move,
.request-row-move {
  transition: transform 320ms var(--ease-out);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--duration-base) var(--ease-out);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.request-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 12px 0;
}

.request-row + .request-row {
  border-top: 1px solid var(--border-hairline);
}

.request-text {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.request-text .person-name {
  flex: none;
  color: inherit;
  text-decoration: none;
}

.request-text .person-name:hover {
  color: var(--primary);
}

.request-text .meta {
  margin: 0;
}

.request-actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.pill-ghost,
.pill-solid {
  min-height: 2.125rem;
  padding: 0 16px;
  border: 0;
  border-radius: var(--radius-pill);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.pill-ghost {
  background-color: color-mix(in srgb, var(--text) 6%, var(--surface));
  color: var(--text-muted);
}

.pill-ghost:hover:not(:disabled) {
  background-color: color-mix(in srgb, var(--text) 10%, var(--surface));
  color: var(--text);
}

.pill-solid {
  background-color: var(--primary);
  box-shadow: var(--shadow-primary);
  color: var(--surface);
}

.pill-solid:hover:not(:disabled) {
  background-color: var(--primary-hover);
  box-shadow: var(--shadow-primary-lift);
  transform: translateY(-1px);
}

.pill-ghost:active:not(:disabled),
.pill-solid:active:not(:disabled) {
  transform: translateY(0);
  transition-duration: 80ms;
}

.pill-ghost:focus-visible,
.pill-solid:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.pill-ghost:disabled,
.pill-solid:disabled {
  opacity: 0.6;
  cursor: progress;
}

@media (max-width: 47.99rem) {
  .group-detail {
    min-height: 0;
    padding: 20px 16px 40px;
  }

  .group-head {
    padding: 18px;
  }

  .head-actions {
    align-items: stretch;
    width: 100%;
  }

  .creator-tools {
    align-items: stretch;
  }

  .tools-row {
    justify-content: space-between;
  }

  .chat-frame {
    height: clamp(22rem, 52vh, 30rem);
  }

  .secondary {
    grid-template-columns: minmax(0, 1fr);
  }

  .slide-over {
    width: 100%;
    border-radius: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .members-pill,
  .tool-button,
  .join-pill,
  .plus-button,
  .plus-button :deep(.nav-icon),
  .submit-pill,
  .panel-head,
  .panel-chevron,
  .icon-button,
  .invite-pill,
  .pill-ghost,
  .pill-solid,
  .slide-over,
  .slide-over::backdrop,
  .pop-enter-active,
  .pop-leave-active,
  .collapse-enter-active,
  .collapse-leave-active,
  .invite-row.invite-row-leave-active,
  .request-row.request-row-leave-active,
  .invite-row-move,
  .request-row-move,
  .fade-enter-active,
  .fade-leave-active {
    transition-duration: 1ms;
  }

  .members-pill:hover,
  .tool-button:hover,
  .join-pill:hover:not(:disabled),
  .plus-button:hover,
  .submit-pill:hover:not(:disabled),
  .invite-pill:hover:not(:disabled),
  .pill-solid:hover:not(:disabled),
  .icon-button:hover {
    transform: none;
  }
}
</style>
