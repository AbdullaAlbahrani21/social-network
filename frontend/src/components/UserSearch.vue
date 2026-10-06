<script setup>
import { computed, onUnmounted, reactive, ref, useId, watch } from 'vue'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'
import { authorInitials, authorName, mediaUrl } from '../utils/display'
import NavIcon from './NavIcon.vue'

const props = defineProps({
  label: { type: String, default: 'Search people' },
  unavailableReason: { type: Function, default: null },
  openableAnyway: { type: Function, default: null },
  inline: { type: Boolean, default: false },
})

const emit = defineEmits(['select', 'engaged'])

const auth = useAuthStore()

const query = ref('')
const results = ref([])
const searching = ref(false)
const error = ref(null)
const searchedTerm = ref('')
const open = ref(false)
const focused = ref(false)
const activeIndex = ref(-1)
const failedAvatars = reactive(new Set())
const inputEl = ref(null)

const listId = useId()

const engaged = computed(() => focused.value || query.value.trim() !== '')
watch(engaged, (value) => emit('engaged', value))

// Requests go out 300ms after the last keystroke, and a token drops a slower response for older text so it can't overwrite a newer one.
let loadToken = 0
let requestedTerm = ''
let debounceTimer = null

async function search() {
  const token = ++loadToken
  const term = query.value.trim()
  requestedTerm = term
  searching.value = true
  error.value = null

  try {
    const users = await api.get(`/api/users?search=${encodeURIComponent(term)}`)
    if (token !== loadToken) return
    results.value = users
    activeIndex.value = -1
  } catch (e) {
    if (token !== loadToken) return
    error.value = e.message
    results.value = []
  } finally {
    if (token === loadToken) {
      searching.value = false
      searchedTerm.value = term
    }
  }
}

watch(query, (value) => {
  clearTimeout(debounceTimer)
  const term = value.trim()

  // The endpoint answers a blank search with a 400, so one is never sent.
  if (!term) {
    loadToken++
    requestedTerm = ''
    results.value = []
    searching.value = false
    error.value = null
    searchedTerm.value = ''
    activeIndex.value = -1
    return
  }

  open.value = true
  if (term === requestedTerm && !error.value) return
  debounceTimer = setTimeout(search, 300)
})

onUnmounted(() => clearTimeout(debounceTimer))

// Splits the API's "first last" at the first space; a first name containing a space gives the wrong initials, not the wrong name.
function asAuthor(user) {
  const space = user.name.indexOf(' ')
  return {
    first_name: space === -1 ? user.name : user.name.slice(0, space),
    last_name: space === -1 ? '' : user.name.slice(space + 1),
    nickname: user.nickname,
  }
}

const items = computed(() =>
  results.value.map((user) => {
    const author = asAuthor(user)
    const reason = props.unavailableReason?.(user) ?? null
    return {
      user,
      name: authorName(author),
      fullName: user.nickname ? user.name : null,
      initials: authorInitials(author),
      isSelf: user.id === auth.user?.id,
      reason,
      openable: !reason || (props.openableAnyway?.(user) ?? false),
    }
  }),
)

const showPanel = computed(
  () =>
    (props.inline || open.value) &&
    query.value.trim() !== '' &&
    (searching.value || searchedTerm.value !== ''),
)

function optionId(index) {
  return `${listId}-${index}`
}

function choose(item) {
  if (!item.openable) return
  emit('select', item.user)
  query.value = ''
  open.value = false
  if (props.inline) inputEl.value?.blur()
}

function move(step) {
  const slots = items.value.length + 1
  let next = activeIndex.value
  for (let i = 0; i < slots; i++) {
    next = ((next + 1 + step + slots) % slots) - 1
    if (next === -1 || items.value[next].openable) {
      activeIndex.value = next
      return
    }
  }
}

function onKeydown(event) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    if (!items.value.length) return
    event.preventDefault()
    open.value = true
    move(event.key === 'ArrowDown' ? 1 : -1)
  } else if (event.key === 'Enter') {
    const item = items.value[activeIndex.value]
    if (showPanel.value && item) {
      event.preventDefault()
      choose(item)
    }
  } else if (event.key === 'Escape') {
    if (props.inline) {
      event.preventDefault()
      if (query.value) query.value = ''
      else inputEl.value?.blur()
    } else if (showPanel.value) {
      event.preventDefault()
      open.value = false
      activeIndex.value = -1
    } else if (query.value) {
      event.preventDefault()
      query.value = ''
    }
  }
}

function onFocus() {
  open.value = true
  focused.value = true
}

// Results keep focus in the input when pressed (mousedown.prevent), so focus leaving the component means the viewer has moved on.
function onFocusOut(event) {
  if (!event.currentTarget.contains(event.relatedTarget)) {
    open.value = false
    focused.value = false
    activeIndex.value = -1
  }
}
</script>

<template>
  <div class="user-search" :class="{ 'user-search-inline': inline }" @focusout="onFocusOut">
    <div class="search-field">
      <NavIcon name="search" />
      <input
        ref="inputEl"
        v-model="query"
        type="search"
        maxlength="100"
        class="text-input search-input"
        role="combobox"
        autocomplete="off"
        spellcheck="false"
        aria-autocomplete="list"
        :placeholder="label"
        :aria-label="label"
        :aria-expanded="showPanel ? 'true' : 'false'"
        :aria-controls="showPanel && items.length ? listId : undefined"
        :aria-activedescendant="showPanel && activeIndex >= 0 ? optionId(activeIndex) : undefined"
        @focus="onFocus"
        @keydown="onKeydown"
      />
    </div>

    <p v-if="inline && focused && !query.trim()" class="panel-message search-hint">
      Search for anyone by name or nickname.
    </p>

    <Transition name="panel">
      <div v-if="showPanel" class="search-panel">
        <ul v-if="items.length" :id="listId" class="results" role="listbox" :aria-label="`${label} results`">
          <li
            v-for="(item, index) in items"
            :id="optionId(index)"
            :key="item.user.id"
            class="result"
            :class="{ 'is-active': index === activeIndex, 'is-unavailable': item.reason }"
            role="option"
            :aria-selected="index === activeIndex ? 'true' : 'false'"
            :aria-disabled="item.openable ? undefined : 'true'"
            @mousedown.prevent
            @mousemove="activeIndex = item.openable ? index : -1"
            @click="choose(item)"
          >
            <img
              v-if="item.user.avatarPath && !failedAvatars.has(item.user.id)"
              :src="mediaUrl(item.user.avatarPath)"
              alt=""
              class="avatar"
              @error="failedAvatars.add(item.user.id)"
            />
            <span v-else class="avatar avatar-initials" aria-hidden="true">{{ item.initials }}</span>

            <span class="result-text">
              <span class="result-name">{{ item.name }}</span>
              <span v-if="item.fullName" class="result-detail">{{ item.fullName }}</span>
            </span>

            <span v-if="item.reason" class="result-tag">{{ item.reason }}</span>
            <span v-else-if="item.isSelf" class="result-tag">You</span>
          </li>
        </ul>

        <p v-else-if="error" class="panel-message panel-error" role="alert">{{ error }}</p>
        <p v-else-if="searching" class="panel-message" role="status">Searching…</p>
        <p v-else class="panel-message" role="status">No one matches “{{ searchedTerm }}”.</p>
      </div>
    </Transition>
  </div>
</template>

<style scoped src="../styles/controls.css"></style>
<style scoped>
.user-search {
  position: relative;
}

.search-field {
  position: relative;
}

.search-field .nav-icon {
  position: absolute;
  top: 50%;
  left: 18px;
  width: 1.125rem;
  height: 1.125rem;
  color: var(--text-placeholder);
  transform: translateY(-50%);
  pointer-events: none;
  transition: color var(--duration-fast) var(--ease-out);
}

.search-field:focus-within .nav-icon {
  color: var(--primary);
}

.search-input {
  min-height: 3rem;
  padding: 0 20px 0 calc(18px + 1.125rem + 10px);
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-pill);
  box-shadow: 0 6px 18px -12px color-mix(in srgb, var(--primary) 30%, transparent);
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

.search-input::-webkit-search-cancel-button {
  cursor: pointer;
  opacity: 0.5;
}

.search-panel {
  position: absolute;
  top: calc(100% + var(--space-2));
  right: 0;
  left: 0;
  z-index: 5;
  max-height: min(22rem, 60vh);
  overflow-y: auto;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: 0 20px 40px -20px color-mix(in srgb, var(--primary) 40%, transparent);
}

.panel-enter-active,
.panel-leave-active {
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.panel-enter-from,
.panel-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

@media (prefers-reduced-motion: reduce) {
  .panel-enter-active,
  .panel-leave-active,
  .search-input,
  .search-field .nav-icon {
    transition-duration: 1ms;
  }
}

.user-search-inline {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.user-search-inline .search-field {
  flex: none;
}

.user-search-inline .search-panel {
  position: static;
  flex: 1;
  min-height: 0;
  max-height: none;
  margin-top: var(--space-2);
  background: none;
  border: 0;
  border-radius: 0;
  box-shadow: none;
}

.user-search-inline .results {
  padding: 0;
}

.search-hint {
  padding-inline: var(--space-1);
}

.results {
  margin: 0;
  padding: var(--space-1);
  list-style: none;
}

.result {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-height: 2.75rem;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.result {
  transition: background-color var(--duration-fast) var(--ease-out);
}

.result.is-active {
  background-color: var(--badge-purple-bg);
}

.result.is-unavailable {
  cursor: default;
}

.avatar {
  display: block;
  flex: none;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 50%;
  object-fit: cover;
}

.avatar-initials {
  display: grid;
  place-items: center;
  background-color: var(--badge-purple-bg);
  color: var(--badge-purple-fg);
  font-size: var(--text-sm);
  font-weight: 700;
}

.is-unavailable .avatar {
  opacity: 0.55;
  filter: grayscale(1);
}

.result-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.3;
}

.result-name,
.result-detail {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-name {
  font-weight: 700;
}

.result-detail {
  color: var(--text-muted);
  font-size: var(--text-sm);
}

.is-unavailable .result-name {
  color: var(--text-muted);
  font-weight: 600;
}

.result-tag {
  flex: none;
  margin-left: auto;
  color: var(--text-muted);
  font-size: var(--text-sm);
  font-weight: 600;
}

.panel-message {
  margin: 0;
  padding: var(--space-3) var(--space-4);
  color: var(--text-muted);
  font-size: var(--text-sm);
}

.panel-error {
  color: var(--error);
}

.panel-error::first-letter {
  text-transform: uppercase;
}
</style>
