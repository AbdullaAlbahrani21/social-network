<script setup>
import { computed, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'
import { authorInitials, authorName, formatTimestamp, mediaUrl } from '../utils/display'
import ConfirmDelete from './ConfirmDelete.vue'
import NavIcon from './NavIcon.vue'

const props = defineProps({
  post: { type: Object, required: true },
  mode: {
    type: String,
    default: 'feed',
    validator: (value) => ['feed', 'detail'].includes(value),
  },
  commentCount: { type: Number, default: null },
})

const emit = defineEmits(['update:post', 'delete', 'comment'])

const auth = useAuthStore()
const router = useRouter()

const name = computed(() => authorName(props.post.author))
const profilePath = computed(() => `/profile/${props.post.user_id}`)
const postPath = computed(() => `/posts/${props.post.id}`)
const isOwn = computed(() => props.post.user_id === auth.user?.id)
const comments = computed(() => props.commentCount ?? props.post.comment_count ?? null)

const avatarFailed = ref(false)
const imageFailed = ref(false)
watch(
  () => props.post.id,
  () => {
    avatarFailed.value = false
    imageFailed.value = false
  },
)

const liking = ref(false)

// Likes or unlikes the post, updating the heart immediately and undoing it if the request fails.
async function toggleLike() {
  if (liking.value) return

  const previous = {
    like_count: props.post.like_count,
    viewer_has_liked: props.post.viewer_has_liked,
  }
  const like = !previous.viewer_has_liked

  emit('update:post', {
    ...props.post,
    viewer_has_liked: like,
    like_count: Math.max(0, previous.like_count + (like ? 1 : -1)),
  })

  liking.value = true
  try {
    const path = `/api/posts/${props.post.id}/like`
    const state = like ? await api.post(path) : await api.del(path)
    emit('update:post', {
      ...props.post,
      like_count: state.like_count,
      viewer_has_liked: state.viewer_has_liked,
    })
  } catch {
    emit('update:post', { ...props.post, ...previous })
  } finally {
    liking.value = false
  }
}

// Opens the post's own page when the card (not a link or button) is clicked in the feed.
function openPost(event) {
  if (props.mode !== 'feed') return
  if (event.target.closest('a, button, input, textarea, select')) return
  // Selecting text in a post isn't asking to open it.
  if (window.getSelection()?.toString()) return
  router.push(postPath.value)
}
</script>

<template>
  <article class="post-card" :class="`post-card-${mode}`" @click="openPost">
    <header class="post-head">
      <RouterLink :to="profilePath" class="avatar-link" tabindex="-1" aria-hidden="true">
        <img
          v-if="post.author.avatar_path && !avatarFailed"
          :src="mediaUrl(post.author.avatar_path)"
          alt=""
          class="avatar"
          @error="avatarFailed = true"
        />
        <span v-else class="avatar avatar-initials">{{ authorInitials(post.author) }}</span>
      </RouterLink>

      <div class="post-meta">
        <RouterLink :to="profilePath" class="author">{{ name }}</RouterLink>
        <time class="post-time" :datetime="post.created_at">{{ formatTimestamp(post.created_at) }}</time>
      </div>

      <ConfirmDelete v-if="isOwn" :key="post.id" noun="post" @confirm="emit('delete', post.id)" />
    </header>

    <p class="post-content">{{ post.content }}</p>

    <img
      v-if="post.image_path && !imageFailed"
      :src="mediaUrl(post.image_path)"
      :alt="`Image posted by ${name}`"
      class="post-image"
      :loading="mode === 'feed' ? 'lazy' : 'eager'"
      @error="imageFailed = true"
    />

    <footer class="post-actions">
      <button
        type="button"
        class="action like-button"
        :class="{ 'is-liked': post.viewer_has_liked }"
        :aria-pressed="post.viewer_has_liked ? 'true' : 'false'"
        :aria-busy="liking ? 'true' : undefined"
        @click.stop="toggleLike"
      >
        <!-- Keyed on the like state, so flipping it remounts the heart and plays its pop or settle. -->
        <Transition name="heart">
          <span :key="post.viewer_has_liked ? 'liked' : 'unliked'" class="heart">
            <NavIcon name="heart" :filled="post.viewer_has_liked" />
          </span>
        </Transition>
        <span class="count">
          <Transition name="count">
            <span :key="post.like_count" class="count-value">{{ post.like_count }}</span>
          </Transition>
        </span>
        <span class="visually-hidden">&nbsp;{{ post.like_count === 1 ? 'like' : 'likes' }}</span>
      </button>

      <RouterLink v-if="mode === 'feed'" :to="postPath" class="action">
        <NavIcon name="messages" />
        <template v-if="comments !== null">
          <span class="count">
            <Transition name="count">
              <span :key="comments" class="count-value">{{ comments }}</span>
            </Transition>
          </span>
          <span class="visually-hidden">&nbsp;{{ comments === 1 ? 'comment' : 'comments' }}</span>
        </template>
        <span v-else>Comments</span>
      </RouterLink>

      <button v-else type="button" class="action" @click="emit('comment')">
        <NavIcon name="messages" />
        <template v-if="comments !== null">
          <span class="count">
            <Transition name="count">
              <span :key="comments" class="count-value">{{ comments }}</span>
            </Transition>
          </span>
          <span class="visually-hidden">&nbsp;{{ comments === 1 ? 'comment' : 'comments' }}</span>
        </template>
        <span v-else>Comments</span>
      </button>
    </footer>
  </article>
</template>

<style scoped>
.post-card {
  padding: 22px 24px 14px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
}

.post-card-feed {
  cursor: pointer;
  transition:
    transform var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out),
    border-color var(--duration-base) var(--ease-out);
}

.post-card-feed:hover {
  border-color: var(--border);
  box-shadow: 0 18px 36px -18px color-mix(in srgb, var(--primary) 38%, transparent);
  transform: translateY(-3px);
}

.post-card-feed:active {
  transform: translateY(-1px);
  transition-duration: 80ms;
}

.post-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2) var(--space-3);
}

.post-head > .confirm-delete {
  align-self: flex-start;
  margin-top: -2px;
}

.avatar-link {
  flex: none;
  text-decoration: none;
  border-radius: 50%;
}

.avatar {
  display: block;
  width: 2.5rem;
  height: 2.5rem;
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

.post-meta {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.3;
}

.author {
  color: var(--text);
  font-size: var(--text-body);
  font-weight: 700;
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out);
}

.author:hover {
  color: var(--primary);
}

.post-time {
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.post-content {
  margin: 14px 0 0;
  color: var(--text);
  font-size: var(--text-body);
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.post-image {
  display: block;
  width: 100%;
  margin-top: 16px;
  background-color: var(--badge-purple-bg);
  border-radius: 16px;
}

/* A fixed height keeps feed cards compact and stops images that load late from moving the page under the reader. */
.post-card-feed .post-image {
  height: 18rem;
  object-fit: cover;
}

.post-card-detail .post-image {
  height: auto;
}

.post-actions {
  display: flex;
  justify-content: space-between;
  margin: 10px -12px 0;
}

.action {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 2.5rem;
  padding: var(--space-2) var(--space-3);
  background: none;
  border: 0;
  border-radius: var(--radius-pill);
  color: var(--text-muted);
  font-size: var(--text-label);
  font-weight: 700;
  text-decoration: none;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.action:hover {
  background-color: var(--badge-purple-bg);
  color: var(--primary);
}

.action .nav-icon {
  width: 1.2rem;
  height: 1.2rem;
}

.like-button.is-liked {
  color: var(--primary);
}

.heart {
  position: relative;
  display: grid;
  place-items: center;
}

.is-liked .heart-enter-active {
  animation: heart-pop 460ms var(--ease-out);
}

.is-liked .heart-enter-active::after {
  content: "";
  position: absolute;
  inset: -6px;
  border: 2px solid var(--primary);
  border-radius: 50%;
  animation: heart-ring 460ms var(--ease-out) forwards;
  pointer-events: none;
}

.like-button:not(.is-liked) .heart-enter-active {
  animation: heart-release 260ms var(--ease-out);
}

.heart-leave-active {
  display: none;
}

@keyframes heart-pop {
  0% { transform: scale(0.6); }
  45% { transform: scale(1.32); }
  70% { transform: scale(0.92); }
  100% { transform: scale(1); }
}

@keyframes heart-ring {
  0% { opacity: 0.55; transform: scale(0.4); }
  100% { opacity: 0; transform: scale(1.35); }
}

@keyframes heart-release {
  0% { transform: scale(0.8); }
  100% { transform: scale(1); }
}

.count {
  position: relative;
  display: inline-grid;
  min-width: 1ch;
  color: var(--text);
}

.like-button.is-liked .count {
  color: var(--primary);
}

.count-value {
  grid-area: 1 / 1;
}

.count-enter-active,
.count-leave-active {
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.count-enter-active {
  animation: count-flash 700ms var(--ease-out);
}

.count-enter-from {
  opacity: 0;
  transform: translateY(70%);
}

.count-leave-to {
  opacity: 0;
  transform: translateY(-70%);
}

@keyframes count-flash {
  0%, 40% { color: var(--primary); }
}

@media (max-width: 47.99rem) {
  .post-card {
    padding: 18px 18px 10px;
  }

  .post-card-feed .post-image {
    height: 14rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .post-card-feed,
  .action,
  .count-enter-active,
  .count-leave-active {
    transition-duration: 1ms;
  }

  .post-card-feed:hover {
    transform: none;
  }

  .heart-enter-active,
  .heart-enter-active::after,
  .count-enter-active {
    animation: none !important;
  }
}
</style>
