<script setup>
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'
import NavIcon from '../components/NavIcon.vue'
import UserSearch from '../components/UserSearch.vue'

const auth = useAuthStore()
const router = useRouter()

const groups = ref([])
const loading = ref(false)
const error = ref(null)
const query = ref('')

const myGroups = ref([])
const myGroupsLoading = ref(false)
const myGroupsError = ref(null)

const browsable = computed(() => groups.value.filter((group) => !group.isMember))

const joinRequests = reactive({})
const idleRequest = { sending: false, sent: false, error: null }

const invitations = ref([])
const invitationsLoading = ref(false)
const invitationsError = ref(null)
const invitationActions = reactive({})
const idleAction = { working: false, error: null }

const invitationsDialog = ref(null)

function joinState(id) {
  return joinRequests[id] || idleRequest
}

function invitationState(invitationId) {
  return invitationActions[invitationId] || idleAction
}

function openInvitations() {
  invitationsDialog.value?.showModal()
}

function closeInvitations() {
  invitationsDialog.value?.close()
}

// A click that lands on the <dialog> itself, not the panel inside it, was on the backdrop.
function onDialogClick(event) {
  if (event.target === invitationsDialog.value) {
    closeInvitations()
  }
}

const createDialog = ref(null)
const newTitle = ref('')
const newDescription = ref('')
const invitees = ref([])
const creating = ref(false)
const createError = ref(null)

const createdGroup = ref(null)
const failedInvites = ref([])

const canCreate = computed(
  () => newTitle.value.trim() !== '' && newDescription.value.trim() !== '' && !creating.value,
)

function personLabel(user) {
  return user.nickname || user.name
}

function inviteeReason(user) {
  if (user.id === auth.user?.id) return 'You'
  if (invitees.value.some((invitee) => invitee.id === user.id)) return 'Added'
  return null
}

function addInvitee(user) {
  if (!invitees.value.some((invitee) => invitee.id === user.id)) {
    invitees.value.push(user)
  }
}

function removeInvitee(userId) {
  invitees.value = invitees.value.filter((invitee) => invitee.id !== userId)
}

function openCreate() {
  newTitle.value = ''
  newDescription.value = ''
  invitees.value = []
  creating.value = false
  createError.value = null
  createdGroup.value = null
  failedInvites.value = []
  createDialog.value?.showModal()
}

function closeCreate() {
  createDialog.value?.close()
}

function onCreateDialogClick(event) {
  if (event.target === createDialog.value) {
    closeCreate()
  }
}

function goToCreatedGroup(group) {
  closeCreate()
  router.push(`/groups/${group.id}`)
}

async function submitCreate() {
  if (!canCreate.value) return

  creating.value = true
  createError.value = null
  failedInvites.value = []

  let group
  try {
    group = await api.post('/api/groups', {
      title: newTitle.value.trim(),
      description: newDescription.value.trim(),
    })
  } catch (e) {
    createError.value = e.message
    creating.value = false
    return
  }

  // allSettled, not all: one rejection must not hide whether the others went through.
  const results = await Promise.allSettled(
    invitees.value.map((user) => api.post(`/api/groups/${group.id}/invite`, { userId: user.id })),
  )
  const failed = invitees.value.filter((_, index) => results[index].status === 'rejected')

  await loadMyGroups()

  creating.value = false

  if (failed.length) {
    createdGroup.value = group
    failedInvites.value = failed.map(personLabel)
    return
  }

  goToCreatedGroup(group)
}

let loadToken = 0

async function loadGroups() {
  const token = ++loadToken
  const search = query.value.trim()
  const path = search
    ? `/api/groups?search=${encodeURIComponent(search)}`
    : '/api/groups'

  loading.value = true
  error.value = null

  try {
    const result = await api.get(path)
    if (token !== loadToken) return
    groups.value = result
  } catch (e) {
    if (token !== loadToken) return
    error.value = e.message
    groups.value = []
  } finally {
    if (token === loadToken) loading.value = false
  }
}

async function loadMyGroups() {
  myGroupsLoading.value = true
  myGroupsError.value = null

  try {
    myGroups.value = await api.get('/api/groups?member=true')
  } catch (e) {
    myGroupsError.value = e.message
    myGroups.value = []
  } finally {
    myGroupsLoading.value = false
  }
}

async function loadInvitations() {
  invitationsLoading.value = true
  invitationsError.value = null

  try {
    invitations.value = await api.get('/api/my-invitations')
  } catch (e) {
    invitationsError.value = e.message
    invitations.value = []
  } finally {
    invitationsLoading.value = false
  }
}

async function requestToJoin(group) {
  joinRequests[group.id] = { sending: true, sent: false, error: null }

  try {
    await api.post(`/api/groups/${group.id}/join-request`, {})
    joinRequests[group.id] = { sending: false, sent: true, error: null }
  } catch (e) {
    joinRequests[group.id] = { sending: false, sent: false, error: e.message }
  }
}

async function respondToInvitation(invitation, action) {
  const id = invitation.invitationId
  invitationActions[id] = { working: true, error: null }

  try {
    await api.post(`/api/group-invitations/${id}/${action}`, {})
  } catch (e) {
    invitationActions[id] = { working: false, error: e.message }
    return
  }

  invitations.value = invitations.value.filter((item) => item.invitationId !== id)
  delete invitationActions[id]

  if (action === 'accept') {
    await Promise.all([loadGroups(), loadMyGroups()])
  }

  if (invitations.value.length === 0) {
    closeInvitations()
  }
}

let debounceTimer = null

watch(query, () => {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(loadGroups, 300)
})

watch(
  () => auth.isLoggedIn,
  (loggedIn) => {
    if (loggedIn) {
      loadGroups()
      loadMyGroups()
      loadInvitations()
    }
  },
  { immediate: true },
)

onUnmounted(() => clearTimeout(debounceTimer))

const TONES = ['tone-purple', 'tone-pink', 'tone-muted']

function toneFor(index) {
  return TONES[index % TONES.length]
}

const TREATMENTS = ['treat-band', 'treat-edge', 'treat-wash']

function treatmentFor(index) {
  return TREATMENTS[index % TREATMENTS.length]
}

// Mirrors pkg/groups' MaxGroupMembers: the API sends no cap, and the server is still what refuses an eleventh member.
const MAX_GROUP_MEMBERS = 10

const NEW_FOR_MS = 7 * 24 * 60 * 60 * 1000

const FILTERS = [
  { id: 'all', label: 'All' },
  { id: 'room', label: 'Has room' },
  { id: 'new', label: 'New' },
]

const filter = ref('all')

const visibleGroups = computed(() => {
  switch (filter.value) {
    case 'room':
      return browsable.value.filter((group) => group.memberCount < MAX_GROUP_MEMBERS)
    case 'new':
      return browsable.value.filter(
        (group) => Date.now() - new Date(group.createdAt).getTime() < NEW_FOR_MS,
      )
    default:
      return browsable.value
  }
})
</script>

<template>
  <main class="groups-page">
    <p v-if="auth.loading && !auth.isLoggedIn" class="status">Checking your session...</p>

    <p v-else-if="!auth.isLoggedIn" class="status">
      You need to be logged in to browse groups.
      <RouterLink to="/login">Log in</RouterLink>
    </p>

    <template v-else>
      <section class="hero">
        <span class="blob blob-1" aria-hidden="true"></span>
        <span class="blob blob-2" aria-hidden="true"></span>
        <span class="blob blob-3" aria-hidden="true"></span>

        <div class="hero-text">
          <h1 class="page-title">Groups</h1>
          <p class="hero-subtitle">
            Join something already running, or start one of your own.
          </p>
        </div>

        <div class="hero-actions">
          <button
            type="button"
            class="action-tile"
            aria-haspopup="dialog"
            @click="openInvitations"
          >
            <span class="action-icon" aria-hidden="true"><NavIcon name="mail" /></span>
            <span class="action-text">
              <span class="action-label">Invitations</span>
              <span class="action-sub">
                {{ invitations.length ? 'Waiting for you' : 'Nothing waiting' }}
              </span>
            </span>
            <span v-if="invitations.length" class="invitations-count">
              {{ invitations.length }}
            </span>
          </button>

          <button
            type="button"
            class="action-tile"
            aria-haspopup="dialog"
            @click="openCreate"
          >
            <span class="action-icon" aria-hidden="true"><NavIcon name="plus" /></span>
            <span class="action-text">
              <span class="action-label">Create group</span>
              <span class="action-sub">Start something new</span>
            </span>
          </button>
        </div>
      </section>

      <section class="mine" aria-labelledby="mine-title">
        <h2 id="mine-title" class="section-title">Your groups</h2>

        <p v-if="myGroupsLoading" class="status mine-message">Loading...</p>

        <p v-else-if="myGroupsError" class="error mine-message" role="alert">
          {{ myGroupsError }}
        </p>

        <p v-else-if="myGroups.length === 0" class="status mine-message">
          You are not in any groups yet.
        </p>

        <ul v-else class="mine-row">
          <li v-for="(group, index) in myGroups" :key="group.id">
            <RouterLink :to="`/groups/${group.id}`" class="mine-tile">
              <span class="mine-badge" :class="toneFor(index)" aria-hidden="true">
                <NavIcon name="groups" />
              </span>
              <span class="mine-name">{{ group.title }}</span>
              <span v-if="group.role === 'creator'" class="role-pill">Creator</span>
            </RouterLink>
          </li>
        </ul>
      </section>

      <section class="browse" aria-labelledby="browse-title">
        <h2 id="browse-title" class="section-title">Discover groups</h2>

        <form class="browse-search" @submit.prevent="loadGroups">
          <label for="groups-search" class="visually-hidden">Search groups</label>
          <NavIcon name="search" class="search-icon" />
          <input
            id="groups-search"
            v-model="query"
            type="search"
            maxlength="100"
            class="search-input"
            placeholder="Search groups by title or description"
          />
        </form>

        <div class="filter-chips" role="group" aria-label="Filter groups">
          <button
            v-for="option in FILTERS"
            :key="option.id"
            type="button"
            class="chip"
            :class="{ 'is-active': filter === option.id }"
            :aria-pressed="filter === option.id"
            @click="filter = option.id"
          >
            {{ option.label }}
          </button>
        </div>

        <p v-if="loading" class="status browse-message">Loading groups...</p>

        <p v-else-if="error" class="error browse-message" role="alert">{{ error }}</p>

        <p v-else-if="browsable.length === 0 && query.trim()" class="status browse-message">
          No groups match "{{ query.trim() }}".
        </p>

        <p v-else-if="browsable.length === 0" class="status browse-message">
          No groups to join right now.
        </p>

        <p v-else-if="visibleGroups.length === 0" class="status browse-message">
          No groups under this filter.
          <button type="button" class="link-button" @click="filter = 'all'">Show all</button>
        </p>

        <TransitionGroup v-else tag="ul" name="card" class="group-cards">
          <li
            v-for="(group, index) in visibleGroups"
            :key="group.id"
            class="group-card"
            :class="treatmentFor(index)"
          >
            <div class="card-head" aria-hidden="true">
              <span class="card-icon"><NavIcon name="groups" /></span>
            </div>

            <div class="card-body">
              <h3 class="card-title">
                <RouterLink :to="`/groups/${group.id}`">{{ group.title }}</RouterLink>
              </h3>

              <p class="card-description">{{ group.description }}</p>

              <div class="card-foot">
                <span class="card-members">
                  {{ group.memberCount }}
                  {{ group.memberCount === 1 ? 'member' : 'members' }}
                </span>

                <button
                  type="button"
                  class="join-pill"
                  :class="{ 'is-sent': joinState(group.id).sent }"
                  :disabled="joinState(group.id).sending || joinState(group.id).sent"
                  @click="requestToJoin(group)"
                >
                  {{
                    joinState(group.id).sent
                      ? 'Request sent'
                      : joinState(group.id).sending
                        ? 'Sending...'
                        : 'Request to join'
                  }}
                </button>
              </div>

              <p v-if="joinState(group.id).error" class="error card-error" role="alert">
                {{ joinState(group.id).error }}
              </p>
            </div>
          </li>
        </TransitionGroup>
      </section>

      <dialog
        ref="invitationsDialog"
        class="slide-over"
        aria-labelledby="invitations-dialog-title"
        @click="onDialogClick"
      >
        <div class="dialog-panel">
          <span class="panel-accent" aria-hidden="true"></span>

          <header class="dialog-head">
            <h2 id="invitations-dialog-title" class="dialog-title">Invitations</h2>
            <button
              type="button"
              class="icon-button"
              aria-label="Close"
              @click="closeInvitations"
            >
              <NavIcon name="close" />
            </button>
          </header>

          <p v-if="invitationsLoading" class="status dialog-message">Loading invitations...</p>

          <p v-else-if="invitationsError" class="error dialog-message" role="alert">
            {{ invitationsError }}
          </p>

          <p v-else-if="invitations.length === 0" class="status dialog-message">
            No pending invitations.
          </p>

          <ul v-else class="invitation-list">
            <li
              v-for="invitation in invitations"
              :key="invitation.invitationId"
              class="invitation-row"
            >
              <span class="invitation-badge" aria-hidden="true">
                <NavIcon name="groups" />
              </span>

              <div class="invitation-text">
                <span class="invitation-group">{{ invitation.groupTitle }}</span>

                <span v-if="invitation.inviterName" class="invitation-inviter">
                  Invited by {{ invitation.inviterName }}
                </span>
              </div>

              <div class="invitation-actions">
                <button
                  type="button"
                  class="pill-ghost"
                  :disabled="invitationState(invitation.invitationId).working"
                  @click="respondToInvitation(invitation, 'decline')"
                >
                  Decline
                </button>

                <button
                  type="button"
                  class="pill-solid"
                  :disabled="invitationState(invitation.invitationId).working"
                  @click="respondToInvitation(invitation, 'accept')"
                >
                  Accept
                </button>
              </div>

              <p
                v-if="invitationState(invitation.invitationId).error"
                class="error invitation-error"
                role="alert"
              >
                {{ invitationState(invitation.invitationId).error }}
              </p>
            </li>
          </ul>
        </div>
      </dialog>

      <dialog
        ref="createDialog"
        class="slide-over create-slide-over"
        aria-labelledby="create-dialog-title"
        @click="onCreateDialogClick"
      >
        <div class="dialog-panel">
          <span class="panel-accent" aria-hidden="true"></span>

          <header class="dialog-head">
            <h2 id="create-dialog-title" class="dialog-title">Create group</h2>
            <button type="button" class="icon-button" aria-label="Close" @click="closeCreate">
              <NavIcon name="close" />
            </button>
          </header>

          <div v-if="createdGroup" class="create-body">
            <p class="error" role="alert">
              {{ failedInvites.length === 1 ? 'One invitation' : `${failedInvites.length} invitations` }}
              could not be sent: {{ failedInvites.join(', ') }}.
            </p>

            <p class="status">
              "{{ createdGroup.title }}" was created. You can invite them again from the
              group's own page.
            </p>

            <div class="create-actions">
              <button type="button" class="pill-solid" @click="goToCreatedGroup(createdGroup)">
                Go to group
              </button>
            </div>
          </div>

          <form v-else class="create-body" @submit.prevent="submitCreate">
            <div class="field">
              <label for="new-group-title" class="field-label">Title</label>
              <input
                id="new-group-title"
                v-model="newTitle"
                type="text"
                class="text-input"
                autocomplete="off"
                maxlength="100"
                :disabled="creating"
              />
            </div>

            <div class="field">
              <label for="new-group-description" class="field-label">Description</label>
              <textarea
                id="new-group-description"
                v-model="newDescription"
                class="text-input"
                rows="3"
                maxlength="1000"
                :disabled="creating"
              ></textarea>
            </div>

            <div class="field">
              <span class="field-label">Invite people (optional)</span>

              <UserSearch
                class="invite-search"
                label="Search people by name or nickname"
                inline
                :unavailable-reason="inviteeReason"
                @select="addInvitee"
              />

              <!-- The list stays mounted: Vue runs no enter or leave transition for a child that appears with its own TransitionGroup. -->
              <TransitionGroup tag="ul" name="chip" class="invitee-chips">
                <li v-for="invitee in invitees" :key="invitee.id" class="invitee-chip">
                  <span class="invitee-name">{{ personLabel(invitee) }}</span>
                  <button
                    type="button"
                    class="chip-remove"
                    :aria-label="`Remove ${personLabel(invitee)}`"
                    :disabled="creating"
                    @click="removeInvitee(invitee.id)"
                  >
                    <NavIcon name="close" />
                  </button>
                </li>
              </TransitionGroup>

              <p v-if="!invitees.length" class="status create-hint">No one invited yet.</p>
            </div>

            <p v-if="createError" class="error" role="alert">{{ createError }}</p>

            <div class="create-actions">
              <button type="submit" class="pill-solid" :disabled="!canCreate">
                {{ creating ? 'Creating...' : 'Create group' }}
              </button>
            </div>
          </form>
        </div>
      </dialog>
    </template>
  </main>
</template>

<style scoped src="../styles/controls.css"></style>

<style scoped>
.groups-page {
  max-width: 60rem;
  margin: 0 auto;
  padding-block: 40px var(--space-7);
}

.section-title {
  margin: 0 0 14px;
  font-size: var(--text-h2);
  font-weight: 800;
}

.hero {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-5) var(--space-6);
  margin-bottom: var(--space-6);
  padding: 30px 34px;
  overflow: hidden;
  background: var(--panel-gradient);
  border-radius: var(--radius-lg);
  box-shadow: 0 24px 50px -28px color-mix(in srgb, var(--primary) 65%, transparent);
}

.blob {
  position: absolute;
  pointer-events: none;
}

.blob-1 {
  left: -12%;
  bottom: -60%;
  width: 46%;
  height: 190%;
  background-color: rgb(255 255 255 / 0.1);
  border-radius: 48% 52% 40% 60% / 38% 44% 56% 62%;
  transform: rotate(-8deg);
}

.blob-2 {
  left: 34%;
  top: -70%;
  width: 40%;
  height: 210%;
  background-color: color-mix(in srgb, var(--notification-dot) 16%, transparent);
  border-radius: 58% 42% 50% 50% / 30% 40% 60% 70%;
  transform: rotate(10deg);
}

.blob-3 {
  right: -14%;
  top: -80%;
  width: 52%;
  height: 170%;
  background-color: rgb(255 255 255 / 0.08);
  border-radius: 50% 50% 45% 55% / 55% 45% 55% 45%;
}

.hero-text {
  position: relative;
  min-width: 0;
}

.page-title {
  margin: 0;
  color: var(--surface);
}

.hero-subtitle {
  margin: 6px 0 0;
  color: rgb(255 255 255 / 0.82);
  font-size: var(--text-body);
}

.hero-actions {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
}

.action-tile {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 11.5rem;
  padding: 12px 18px;
  background-color: rgb(255 255 255 / 0.16);
  backdrop-filter: blur(10px);
  border: 1px solid rgb(255 255 255 / 0.28);
  border-radius: var(--radius-md);
  color: var(--surface);
  text-align: left;
  cursor: pointer;
  transition:
    background-color var(--duration-base) var(--ease-out),
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.action-tile:hover {
  background-color: rgb(255 255 255 / 0.26);
  border-color: rgb(255 255 255 / 0.45);
  box-shadow: 0 14px 28px -14px color-mix(in srgb, var(--text) 45%, transparent);
  transform: translateY(-2px);
}

.action-tile:active {
  transform: translateY(0);
  transition-duration: 80ms;
}

.action-tile:focus-visible {
  outline: 2px solid var(--surface);
  outline-offset: 2px;
}

.action-icon {
  display: grid;
  flex: none;
  place-items: center;
  width: 2.25rem;
  height: 2.25rem;
  background-color: rgb(255 255 255 / 0.22);
  border-radius: var(--radius-pill);
}

.action-icon :deep(.nav-icon) {
  width: 1.1rem;
  height: 1.1rem;
}

.action-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.action-label {
  font-size: var(--text-label);
  font-weight: 700;
}

.action-sub {
  color: rgb(255 255 255 / 0.75);
  font-size: var(--text-meta);
}

.invitations-count {
  flex: none;
  min-width: 1.4rem;
  margin-left: auto;
  padding: 0 6px;
  background-color: var(--notification-dot);
  border-radius: var(--radius-pill);
  color: var(--text);
  font-size: 0.7rem;
  font-weight: 800;
  line-height: 1.4rem;
  text-align: center;
}

.mine {
  margin-bottom: var(--space-6);
}

.mine-message {
  margin: 0;
}

.mine-row {
  display: flex;
  gap: 12px;
  margin: 0 -4px;
  padding: 4px;
  overflow-x: auto;
  overscroll-behavior-x: contain;
  scroll-snap-type: x proximity;
  list-style: none;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}

.mine-row::-webkit-scrollbar {
  height: 6px;
}

.mine-row::-webkit-scrollbar-thumb {
  background-color: var(--border);
  border-radius: var(--radius-pill);
}

.mine-row > li {
  flex: none;
  scroll-snap-align: start;
}

.mine-tile {
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 17rem;
  padding: 10px 16px 10px 10px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-pill);
  box-shadow: var(--shadow-card);
  color: var(--text);
  text-decoration: none;
  transition:
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.mine-tile:hover {
  border-color: var(--border);
  box-shadow: 0 16px 30px -18px color-mix(in srgb, var(--primary) 40%, transparent);
  transform: translateY(-2px);
}

.mine-tile:active {
  transform: translateY(0);
  transition-duration: 80ms;
}

.mine-tile:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.mine-badge {
  display: grid;
  flex: none;
  place-items: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: var(--radius-pill);
}

.mine-badge :deep(.nav-icon) {
  width: 1.1rem;
  height: 1.1rem;
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

.mine-name {
  overflow: hidden;
  font-size: var(--text-label);
  font-weight: 700;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.role-pill {
  flex: none;
  padding: 2px 9px;
  background-color: var(--badge-purple-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-purple-fg);
  font-size: 0.7rem;
  font-weight: 800;
}

.browse-search {
  position: relative;
  margin-bottom: 14px;
}

.search-icon {
  position: absolute;
  top: 50%;
  left: 18px;
  width: 1.125rem;
  height: 1.125rem;
  color: var(--text-placeholder);
  transform: translateY(-50%);
  pointer-events: none;
  transition: color var(--duration-base) var(--ease-out);
}

.browse-search:focus-within .search-icon {
  color: var(--primary);
}

.search-input {
  width: 100%;
  min-height: 3rem;
  padding: 0 20px 0 calc(18px + 1.125rem + 10px);
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

.filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-bottom: 18px;
}

.chip {
  min-height: 2rem;
  padding: 0 16px;
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
    color var(--duration-fast) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.chip:hover:not(.is-active) {
  border-color: var(--border-strong);
  color: var(--text);
}

.chip:active {
  transform: scale(0.97);
  transition-duration: 80ms;
}

.chip:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.chip.is-active {
  background-color: var(--primary);
  border-color: var(--primary);
  box-shadow: var(--shadow-primary);
  color: var(--surface);
}

.browse-message {
  margin-top: var(--space-4);
}

.link-button {
  padding: 0;
  background: none;
  border: 0;
  color: var(--primary);
  font: inherit;
  font-weight: 700;
  text-decoration: underline;
  cursor: pointer;
}

.group-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
  gap: 16px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.group-card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
  transition:
    transform var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    border-color var(--duration-base) var(--ease-out);
}

.group-card:hover {
  border-color: var(--border);
  box-shadow: 0 18px 36px -18px color-mix(in srgb, var(--primary) 38%, transparent);
  transform: translateY(-3px);
}

.card-head {
  position: relative;
  display: flex;
  align-items: flex-end;
  height: 4rem;
  padding: 0 18px;
}

.card-icon {
  display: grid;
  place-items: center;
  width: 2.5rem;
  height: 2.5rem;
  margin-bottom: -1.25rem;
  border: 2px solid var(--surface);
  border-radius: var(--radius-pill);
}

.card-icon :deep(.nav-icon) {
  width: 1.15rem;
  height: 1.15rem;
}

.treat-band .card-head {
  background: var(--panel-gradient);
}

.treat-band .card-icon {
  background-color: var(--surface);
  color: var(--primary);
}

.treat-edge .card-head {
  background-color: var(--badge-pink-bg);
  box-shadow: inset 0 4px 0 var(--badge-pink-fg);
}

.treat-edge .card-icon {
  background-color: var(--surface);
  color: var(--badge-pink-fg);
}

.treat-wash .card-head {
  background-color: var(--badge-muted-bg);
}

.treat-wash .card-icon {
  background-color: var(--surface);
  color: var(--badge-muted-fg);
}

.card-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  padding: 26px 18px 18px;
}

.card-title {
  margin: 0 0 4px;
  font-size: var(--text-card-title);
  font-weight: 700;
}

.card-title a {
  color: inherit;
  text-decoration: none;
}

.card-title a:hover {
  text-decoration: underline;
}

.card-title a:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
  border-radius: var(--radius-xs);
}

.card-description {
  margin: 0 0 16px;
  overflow: hidden;
  color: var(--text-muted);
  font-size: var(--text-meta);
  white-space: nowrap;
  text-overflow: ellipsis;
}

.card-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin-top: auto;
}

.card-members {
  padding: 3px 10px;
  background-color: var(--badge-muted-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-muted-fg);
  font-size: var(--text-meta);
  font-weight: 700;
}

.join-pill {
  min-height: 2.125rem;
  padding: 0 16px;
  background-color: var(--surface);
  border: 1px solid var(--primary);
  border-radius: var(--radius-pill);
  color: var(--primary);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-base) var(--ease-out),
    border-color var(--duration-base) var(--ease-out),
    color var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.join-pill:hover:not(:disabled) {
  background-color: color-mix(in srgb, var(--primary) 10%, var(--surface));
  transform: translateY(-1px);
}

.join-pill:active:not(:disabled) {
  transform: translateY(0);
  transition-duration: 80ms;
}

.join-pill:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.join-pill:disabled {
  cursor: default;
}

.join-pill.is-sent {
  background-color: var(--badge-muted-bg);
  border-color: transparent;
  color: var(--badge-muted-fg);
}

.card-error {
  margin-top: 12px;
}

/* Doubled class: TransitionGroup puts -move on a leaving element too, and its transition would otherwise drop opacity. */
.group-card.card-enter-active,
.group-card.card-leave-active {
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.card-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.card-leave-to {
  opacity: 0;
  transform: scale(0.97);
}

.card-move {
  transition: transform 320ms var(--ease-out);
}

/* allow-discrete keeps the dialog displayed for the length of the transition, so closing slides rather than snapping. */
.slide-over {
  width: min(27rem, 100%);
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

/* showModal() focuses the dialog itself when nothing inside asks for focus, so no ring around its whole edge. */
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

.dialog-message {
  padding: var(--space-5) 26px;
}

.invitation-list {
  margin: 0;
  padding: var(--space-3) 16px var(--space-5) 26px;
  overflow-y: auto;
  overscroll-behavior: contain;
  list-style: none;
}

.invitation-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  padding: 14px 0;
}

.invitation-row + .invitation-row {
  border-top: 1px solid var(--border-hairline);
}

.invitation-badge {
  display: grid;
  flex: none;
  place-items: center;
  width: 2.25rem;
  height: 2.25rem;
  background-color: var(--badge-purple-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-purple-fg);
}

.invitation-badge :deep(.nav-icon) {
  width: 1.1rem;
  height: 1.1rem;
}

.invitation-text {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.invitation-group {
  font-size: var(--text-label);
  font-weight: 700;
}

.invitation-inviter {
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.invitation-actions {
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

.invitation-error {
  flex-basis: 100%;
}

.create-slide-over {
  width: min(30rem, 100%);
}

.create-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--space-4);
  min-height: 0;
  padding: var(--space-5) 16px var(--space-5) 26px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.field-label {
  font-size: var(--text-label);
  font-weight: 700;
}

.invite-search {
  height: auto;
}

.invite-search :deep(.search-panel) {
  max-height: 14rem;
  overflow-y: auto;
}

.invitee-chips {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}

.invitee-chip {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 4px 4px 4px 12px;
  background-color: var(--badge-purple-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-purple-fg);
  font-size: var(--text-label);
  font-weight: 700;
}

/* Doubled class for the same reason the cards carry one; the leaving chip holds its place rather than dropping onto the first line's corner. */
.invitee-chip.chip-enter-active,
.invitee-chip.chip-leave-active {
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.chip-enter-from,
.chip-leave-to {
  opacity: 0;
  transform: scale(0.8);
}

.chip-move {
  transition: transform 320ms var(--ease-out);
}

.chip-remove {
  display: grid;
  place-items: center;
  width: 1.5rem;
  height: 1.5rem;
  padding: 0;
  background: none;
  border: 0;
  border-radius: 50%;
  color: inherit;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    transform var(--duration-fast) var(--ease-out);
}

.chip-remove:hover:not(:disabled) {
  background-color: color-mix(in srgb, var(--badge-purple-fg) 18%, transparent);
  transform: rotate(90deg);
}

.chip-remove:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 1px;
}

.chip-remove:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.chip-remove :deep(.nav-icon) {
  width: 0.875rem;
  height: 0.875rem;
}

.create-hint {
  margin: 0;
  font-size: var(--text-meta);
}

.create-actions {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 47.99rem) {
  .groups-page {
    padding: 20px 16px 40px;
  }

  .hero {
    padding: 22px;
  }

  .hero-actions {
    width: 100%;
  }

  .action-tile {
    flex: 1 1 10rem;
    min-width: 0;
  }

  .group-cards {
    grid-template-columns: minmax(0, 1fr);
  }

  .slide-over {
    width: 100%;
    border-radius: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .action-tile,
  .mine-tile,
  .group-card,
  .join-pill,
  .chip,
  .icon-button,
  .chip-remove,
  .slide-over,
  .slide-over::backdrop,
  .group-card.card-enter-active,
  .group-card.card-leave-active,
  .card-move,
  .invitee-chip.chip-enter-active,
  .invitee-chip.chip-leave-active,
  .chip-move {
    transition-duration: 1ms;
  }

  .action-tile:hover,
  .mine-tile:hover,
  .group-card:hover,
  .join-pill:hover:not(:disabled),
  .pill-solid:hover:not(:disabled) {
    transform: none;
  }

  .icon-button:hover,
  .chip-remove:hover:not(:disabled) {
    transform: none;
  }
}
</style>
