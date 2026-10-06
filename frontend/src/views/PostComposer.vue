<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import { useCreatePost } from '../composables/useCreatePost'
import { useAuthStore } from '../stores/auth'
import { authorInitials, authorName, mediaUrl } from '../utils/display'
import EmojiButton from '../components/EmojiButton.vue'
import NavIcon from '../components/NavIcon.vue'
import SegmentedToggle from '../components/SegmentedToggle.vue'

const router = useRouter()
const auth = useAuthStore()

const { posting, error: submitError, createPost } = useCreatePost()

const content = ref('')
const contentInput = ref(null)

// Grows the text box to fit what has been typed.
// scrollHeight leaves out the 1px borders a border-box height includes.
function autosize() {
  const el = contentInput.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight + 2}px`
}

watch(content, () => nextTick(autosize))

onMounted(() => {
  autosize()
  contentInput.value?.focus()
})

// What the API keeps: pkg/posts/upload.go sniffs the bytes for these three.
const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'image/gif']

const image = ref(null)
const previewUrl = ref('')
const imageError = ref('')
const fileInput = ref(null)
const addPhotoButton = ref(null)

// Sets (or clears) the attached image and its preview URL.
function setImage(file) {
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  image.value = file
  previewUrl.value = file ? URL.createObjectURL(file) : ''
}

// Validates the picked file is a JPEG, PNG or GIF before attaching it.
function onFilePicked(event) {
  const file = event.target.files[0]
  // Cleared, so picking the same file again after removing it still fires.
  event.target.value = ''
  if (!file) return

  if (!ACCEPTED_TYPES.includes(file.type)) {
    imageError.value = `Can't attach "${file.name}". Choose a JPEG, PNG or GIF.`
    return
  }

  imageError.value = ''
  setImage(file)
}

// Drops a file the browser could not display as an image.
function onPreviewError() {
  const name = image.value?.name
  setImage(null)
  imageError.value = `Couldn't read "${name}" as an image. Choose a JPEG, PNG or GIF.`
}

// Removes the attached image and moves focus back to the add-photo button.
async function removeImage() {
  setImage(null)
  imageError.value = ''
  await nextTick()
  addPhotoButton.value?.focus()
}

onBeforeUnmount(() => setImage(null))

const PRIVACY_OPTIONS = [
  { value: 'public', label: 'Public', icon: 'globe' },
  { value: 'almost_private', label: 'Followers Only', icon: 'groups' },
  { value: 'private', label: 'Private', icon: 'lock' },
]

const privacy = ref('public')

const privacyHint = computed(() => {
  if (privacy.value === 'almost_private') return 'Only people who follow you can see this post.'
  if (privacy.value === 'private') return 'Only the followers you pick can see this post.'
  return auth.user?.isPublic === false
    ? 'Your account is private, so only your followers will see this post.'
    : 'Anyone can see this post.'
})

const followers = ref(null)
const followersLoading = ref(false)
const followersError = ref('')
const selected = ref(new Set())
const failedAvatars = ref(new Set())

// Loads the user's followers once, for picking a private post's audience.
async function loadFollowers() {
  if (followers.value || followersLoading.value) return
  followersLoading.value = true
  followersError.value = ''
  try {
    followers.value = await api.get(`/api/users/${auth.user.id}/followers`)
  } catch (e) {
    followersError.value = e.message
  } finally {
    followersLoading.value = false
  }
}

watch(privacy, (value) => {
  if (value === 'private') loadFollowers()
})

// Adds or removes a follower from the private post's audience.
function toggleRecipient(id) {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

// Converts a follower object to the shape the name helper expects.
function asAuthor(person) {
  return { first_name: person.firstName, last_name: person.lastName, nickname: person.nickname }
}

const needsRecipients = computed(() => privacy.value === 'private' && selected.value.size === 0)
const canSubmit = computed(() => content.value.trim() !== '' && !posting.value && !needsRecipients.value)

// Sends the post through useCreatePost, then leaves the composer.
async function submit() {
  if (!canSubmit.value) return
  const post = await createPost({
    content: content.value,
    privacy: privacy.value,
    visibilityUserIds: [...selected.value],
    image: image.value,
  })
  if (post) router.replace(`/posts/${post.id}`)
}
</script>

<template>
  <div class="post-composer">
    <h1 class="composer-title">New post</h1>

    <form class="composer" @submit.prevent="submit">
      <div>
        <label for="post-content" class="visually-hidden">Post text</label>
        <textarea
          id="post-content"
          ref="contentInput"
          v-model="content"
          class="text-input content-input"
          placeholder="What's on your mind?"
          rows="5"
          maxlength="5000"
        ></textarea>
        <EmojiButton target="post-content" />
      </div>

      <div class="photo">
        <input
          ref="fileInput"
          type="file"
          class="visually-hidden photo-input"
          accept="image/jpeg,image/png,image/gif"
          tabindex="-1"
          aria-hidden="true"
          @change="onFilePicked"
        />

        <!-- Not out-in: removeImage focuses Add photo on the next tick, so the incoming control has to be in the DOM at once. -->
        <Transition name="photo-swap">
          <figure v-if="previewUrl" class="photo-preview">
            <img :src="previewUrl" alt="The photo this post will include" @error="onPreviewError" />
            <button type="button" class="photo-remove" aria-label="Remove photo" @click="removeImage">
              <NavIcon name="close" />
            </button>
          </figure>

          <button
            v-else
            ref="addPhotoButton"
            type="button"
            class="photo-add"
            :aria-describedby="imageError ? 'photo-error' : undefined"
            @click="fileInput.click()"
          >
            <span class="photo-add-icon"><NavIcon name="image" /></span>
            <span class="photo-add-text">
              <span class="photo-add-label">Add photo</span>
              <span class="photo-add-hint">JPEG, PNG or GIF</span>
            </span>
          </button>
        </Transition>

        <Transition name="hint">
          <p v-if="imageError" id="photo-error" class="error" role="alert">{{ imageError }}</p>
        </Transition>
      </div>

      <fieldset class="privacy">
        <legend class="privacy-legend">Who can see this</legend>

        <SegmentedToggle
          class="privacy-toggle"
          appearance="outline"
          name="privacy"
          label="Who can see this"
          :options="PRIVACY_OPTIONS"
          :model-value="privacy"
          @select="(value) => (privacy = value)"
        />

        <Transition name="hint" mode="out-in">
          <p :key="privacyHint" class="privacy-hint">{{ privacyHint }}</p>
        </Transition>

        <Transition name="collapse">
          <div v-if="privacy === 'private'" class="audience" role="group" aria-labelledby="audience-title">
            <div class="audience-head">
              <p id="audience-title" class="audience-title">Choose followers</p>
              <p v-if="followers?.length" class="audience-count" aria-live="polite">
                {{ selected.size ? `${selected.size} selected` : 'Pick at least one' }}
              </p>
            </div>

            <p v-if="followersLoading" class="status audience-message">Loading your followers…</p>
            <p v-else-if="followersError" class="error audience-message" role="alert">{{ followersError }}</p>
            <p v-else-if="followers && !followers.length" class="status audience-message">
              You don't have any followers yet, so there's no one to share a private post with.
            </p>

            <ul v-else-if="followers" class="audience-list">
              <li v-for="person in followers" :key="person.id">
                <label class="recipient" :class="{ 'is-selected': selected.has(person.id) }">
                  <input
                    type="checkbox"
                    class="recipient-check"
                    :value="person.id"
                    :checked="selected.has(person.id)"
                    @change="toggleRecipient(person.id)"
                  />
                  <img
                    v-if="person.avatarPath && !failedAvatars.has(person.id)"
                    :src="mediaUrl(person.avatarPath)"
                    alt=""
                    class="avatar"
                    @error="failedAvatars = new Set(failedAvatars).add(person.id)"
                  />
                  <span v-else class="avatar avatar-initials" aria-hidden="true">{{ authorInitials(asAuthor(person)) }}</span>
                  <span class="recipient-text">
                    <span class="recipient-name">{{ authorName(asAuthor(person)) }}</span>
                    <span v-if="person.nickname" class="recipient-detail">{{ person.firstName }} {{ person.lastName }}</span>
                  </span>
                </label>
              </li>
            </ul>
          </div>
        </Transition>
      </fieldset>

      <div class="composer-footer">
        <p v-if="submitError" class="error submit-error" role="alert">{{ submitError }}</p>
        <button
          type="submit"
          class="submit-button"
          :disabled="!canSubmit"
          :aria-busy="posting ? 'true' : undefined"
        >
          <span v-if="posting" class="spinner" aria-hidden="true"></span>
          {{ posting ? 'Posting…' : 'Post' }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped src="../styles/controls.css"></style>

<style scoped>
.post-composer {
  max-width: 42.5rem;
  margin: 0 auto;
  padding-block: 40px var(--space-7);
}

.composer-title {
  margin: 0 0 22px;
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* The hairline stays 1px -- the autosize adds 2px for the borders -- and this outranks controls.css, whose resize: vertical would fight it. */
textarea.content-input {
  display: block;
  min-height: 8.5rem;
  max-height: 60vh;
  padding: 22px 24px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
  font-size: var(--text-body);
  line-height: 1.6;
  resize: none;
  overflow-y: auto;
  transition:
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out);
}

textarea.content-input::placeholder {
  color: var(--text-placeholder);
}

textarea.content-input:hover {
  border-color: var(--border);
}

textarea.content-input:focus-visible {
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
  outline: none;
  box-shadow:
    0 0 0 4px color-mix(in srgb, var(--primary) 14%, transparent),
    var(--shadow-card);
}

.photo {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.photo-add {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  min-height: 4.75rem;
  padding: 16px 18px;
  background-color: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  color: var(--text);
  text-align: left;
  cursor: pointer;
  transition:
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.photo-add:hover {
  border-color: color-mix(in srgb, var(--primary) 35%, var(--border));
  box-shadow: var(--shadow-card);
  transform: translateY(-2px);
}

.photo-add:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.photo-add-icon {
  display: grid;
  flex: none;
  place-items: center;
  width: 2.5rem;
  height: 2.5rem;
  background-color: var(--badge-purple-bg);
  border-radius: 12px;
  color: var(--primary);
  transition: transform var(--duration-base) var(--ease-out);
}

.photo-add:hover .photo-add-icon {
  transform: scale(1.06);
}

.photo-add-icon .nav-icon {
  width: 1.1rem;
  height: 1.1rem;
}

.photo-add-text {
  display: flex;
  flex-direction: column;
  line-height: 1.35;
}

.photo-add-label {
  font-size: var(--text-sm);
  font-weight: 700;
}

.photo-add-hint {
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.photo-preview {
  position: relative;
  margin: 0;
  overflow: hidden;
  background-color: var(--badge-purple-bg);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
}

.photo-preview img {
  display: block;
  width: 100%;
  max-height: 28rem;
  object-fit: contain;
}

.photo-remove {
  position: absolute;
  top: 12px;
  right: 12px;
  display: grid;
  place-items: center;
  width: 2.25rem;
  height: 2.25rem;
  background-color: color-mix(in srgb, var(--text) 62%, transparent);
  border: 0;
  border-radius: 50%;
  color: var(--surface);
  cursor: pointer;
  backdrop-filter: blur(4px);
  transition:
    background-color var(--duration-fast) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.photo-remove:hover {
  background-color: var(--danger);
  transform: rotate(90deg);
}

.photo-remove:focus-visible {
  outline: 2px solid var(--surface);
  outline-offset: 2px;
}

.photo-remove .nav-icon {
  width: 1.1rem;
  height: 1.1rem;
}

.photo-swap-enter-active,
.photo-swap-leave-active {
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.photo-swap-leave-active {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
}

.photo-swap-enter-from {
  opacity: 0;
  transform: scale(0.96);
}

.photo-swap-leave-to {
  opacity: 0;
  transform: scale(0.98);
}

.privacy {
  min-width: 0;
  margin: 4px 0 0;
  padding: 0;
  border: 0;
}

.privacy-legend {
  margin-bottom: 10px;
  padding: 0;
  font-size: var(--text-sm);
  font-weight: 700;
}

.privacy-toggle {
  display: grid;
  width: 100%;
}

.privacy-hint {
  margin: 10px 0 0 4px;
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.hint-enter-active,
.hint-leave-active {
  transition:
    opacity var(--duration-fast) var(--ease-out),
    transform var(--duration-fast) var(--ease-out);
}

.hint-enter-from,
.hint-leave-to {
  opacity: 0;
  transform: translateY(-3px);
}

.audience {
  display: flex;
  flex-direction: column;
  margin-top: 14px;
  padding: 16px 18px 10px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
}

.collapse-enter-active,
.collapse-leave-active {
  overflow: hidden;
  transition:
    opacity var(--duration-base) var(--ease-out),
    max-height 340ms var(--ease-out),
    margin-top 340ms var(--ease-out),
    padding 340ms var(--ease-out);
  max-height: 26rem;
}

.collapse-enter-from,
.collapse-leave-to {
  max-height: 0;
  margin-top: 0;
  padding-block: 0;
  opacity: 0;
}

.audience-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: 8px;
}

.audience-title {
  margin: 0;
  font-size: var(--text-sm);
  font-weight: 700;
}

.audience-count {
  margin: 0;
  padding: 2px 10px;
  background-color: var(--badge-purple-bg);
  border-radius: var(--radius-pill);
  color: var(--badge-purple-fg);
  font-size: var(--text-meta);
  font-weight: 700;
}

.audience-message {
  padding: 6px 0 10px;
}

.audience-list {
  max-height: 20rem;
  margin: 0 -8px;
  padding: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  list-style: none;
}

.recipient {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 3.5rem;
  padding: 8px;
  border-radius: 16px;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-out);
}

.recipient:hover {
  background-color: var(--bg);
}

.recipient.is-selected {
  background-color: var(--badge-purple-bg);
}

.recipient:has(.recipient-check:focus-visible) {
  outline: 2px solid var(--primary);
  outline-offset: -2px;
}

.recipient-check {
  order: 3;
  flex: none;
  width: 1.35rem;
  height: 1.35rem;
  margin: 0 4px 0 auto;
  appearance: none;
  background-color: var(--surface);
  background-position: center;
  background-repeat: no-repeat;
  background-size: 0.8rem;
  border: 1.5px solid var(--border-strong);
  border-radius: 50%;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.recipient-check:checked {
  background-color: var(--primary);
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='white' stroke-width='3.5' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m5 12 5 5 9-10'/%3E%3C/svg%3E");
  border-color: var(--primary);
  animation: check-pop 260ms var(--ease-out);
}

.recipient-check:focus-visible {
  outline: none;
}

@keyframes check-pop {
  50% {
    transform: scale(1.18);
  }
}

.avatar {
  display: block;
  flex: none;
  width: 2.75rem;
  height: 2.75rem;
  border-radius: 50%;
  object-fit: cover;
}

.avatar-initials {
  display: grid;
  place-items: center;
  background-color: var(--badge-purple-bg);
  color: var(--badge-purple-fg);
  font-size: var(--text-label);
  font-weight: 700;
}

.recipient.is-selected .avatar-initials {
  background-color: var(--surface);
}

.recipient-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.3;
}

.recipient-name,
.recipient-detail {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recipient-name {
  font-size: var(--text-sm);
  font-weight: 700;
}

.recipient-detail {
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.composer-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-3);
  margin-top: 4px;
}

.submit-error {
  flex: 1 1 14rem;
}

.submit-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  min-width: 7.5rem;
  min-height: 3rem;
  padding: 0 28px;
  background-color: var(--primary);
  border: 1px solid var(--primary);
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

.submit-button:hover:not(:disabled) {
  background-color: var(--primary-hover);
  box-shadow: var(--shadow-primary-lift);
  transform: translateY(-2px);
}

.submit-button:active:not(:disabled) {
  box-shadow: var(--shadow-primary);
  transform: translateY(0);
  transition-duration: 80ms;
}

.submit-button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 3px;
}

.submit-button:disabled {
  background-color: var(--surface);
  border-color: var(--border);
  box-shadow: none;
  color: var(--text-faint);
  cursor: not-allowed;
}

.submit-button[aria-busy="true"] {
  background-color: var(--primary);
  border-color: var(--primary);
  color: var(--surface);
  cursor: progress;
  animation: busy-pulse 1.1s var(--ease-in-out) infinite;
}

@keyframes busy-pulse {
  50% {
    box-shadow: var(--shadow-primary-lift);
    opacity: 0.85;
  }
}

.spinner {
  width: 1rem;
  height: 1rem;
  border: 2px solid color-mix(in srgb, var(--surface) 35%, transparent);
  border-top-color: var(--surface);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 47.99rem) {
  .post-composer {
    padding: 20px 16px 40px;
  }

  textarea.content-input {
    padding: 18px;
  }

  .privacy-toggle :deep(.segment) {
    padding: 0 6px;
    font-size: 0.75rem;
  }

  .privacy-toggle :deep(.segment .nav-icon) {
    display: none;
  }

  .submit-button {
    width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  textarea.content-input,
  .photo-add,
  .photo-add-icon,
  .photo-remove,
  .photo-swap-enter-active,
  .photo-swap-leave-active,
  .hint-enter-active,
  .hint-leave-active,
  .collapse-enter-active,
  .collapse-leave-active,
  .recipient,
  .recipient-check,
  .submit-button {
    transition-duration: 1ms;
  }

  .photo-add:hover,
  .submit-button:hover:not(:disabled) {
    transform: none;
  }

  .recipient-check:checked,
  .submit-button[aria-busy="true"] {
    animation: none;
  }

  .spinner {
    animation-duration: 2s;
  }
}
</style>
