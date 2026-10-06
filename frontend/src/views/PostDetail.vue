<script setup>
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'
import { usePostComments } from '../composables/usePostComments'
import { authorInitials, authorName, formatTimestamp, mediaUrl } from '../utils/display'
import ConfirmDelete from '../components/ConfirmDelete.vue'
import EmojiButton from '../components/EmojiButton.vue'
import NavIcon from '../components/NavIcon.vue'
import PostCard from '../components/PostCard.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const post = ref(null)
const loading = ref(true)
const loadError = ref('')
const actionError = ref('')

const {
  commentsByPost,
  newCommentText,
  error: commentsError,
  loadComments,
  onCommentImagePicked,
  deleteComment,
  addComment,
} = usePostComments()

const comments = computed(() => (post.value ? commentsByPost.value[post.value.id] : undefined))

const commentInput = ref(null)
// Re-rendering the file input is the only way to clear the file it shows.
const commentFileKey = ref(0)

// Fetches one post, then its comments.
async function loadPost(id) {
  loading.value = true
  loadError.value = ''
  actionError.value = ''
  post.value = null
  try {
    post.value = await api.get(`/api/posts/${id}`)
  } catch (e) {
    loadError.value = e.message
  } finally {
    loading.value = false
  }

  if (post.value) {
    await loadComments(post.value.id)
  }
}

// Going from one post to another reuses this component, so the id is watched rather than loaded once on mount.
watch(
  () => route.params.id,
  (id) => {
    if (route.name === 'post-detail' && id) {
      loadPost(id)
    }
  },
  { immediate: true },
)

// Returns to the previous page, or the feed if there is none.
// With no history.state.back -- opened directly, or in a new tab -- going back would leave the app.
function goBack() {
  if (window.history.state?.back) {
    router.back()
  } else {
    router.push('/')
  }
}

// Deletes the post and returns to the feed.
async function removePost(id) {
  actionError.value = ''
  try {
    await api.del(`/api/posts/${id}`)
    router.push('/')
  } catch (e) {
    actionError.value = e.message
  }
}

// Puts the cursor in the comment box.
function focusCommentInput() {
  commentInput.value?.focus()
}

// Posts the typed comment and resets the image picker on success.
async function submitComment() {
  const id = post.value.id
  const hasText = Boolean((newCommentText.value[id] || '').trim())
  await addComment(id)
  if (hasText && !commentsError.value) {
    commentFileKey.value += 1
  }
}
</script>

<template>
  <div class="post-detail">
    <button type="button" class="back-button" @click="goBack">
      <NavIcon name="back" />
      <span>Back</span>
    </button>

    <p v-if="loading" class="status">Loading post...</p>

    <div v-else-if="loadError" class="unavailable">
      <p class="error" role="alert">{{ loadError }}</p>
      <RouterLink to="/" class="text-link">Go to the feed</RouterLink>
    </div>

    <template v-else-if="post">
      <p v-if="actionError" class="error action-error" role="alert">{{ actionError }}</p>

      <PostCard
        v-model:post="post"
        mode="detail"
        :comment-count="comments ? comments.length : null"
        @delete="removePost"
        @comment="focusCommentInput"
      />

      <section class="comments" aria-labelledby="comments-title">
        <h2 id="comments-title" class="comments-title">Comments</h2>

        <p v-if="commentsError" class="error" role="alert">{{ commentsError }}</p>
        <p v-if="comments && comments.length === 0" class="status">No comments yet.</p>

        <TransitionGroup
          v-if="comments && comments.length"
          tag="ul"
          name="comment-list"
          class="comment-list"
          @before-leave="(el) => (el.style.top = `${el.offsetTop}px`)"
        >
          <li v-for="c in comments" :key="c.id" class="comment">
            <RouterLink :to="`/profile/${c.user_id}`" class="comment-avatar-link" tabindex="-1" aria-hidden="true">
              <img v-if="c.author.avatar_path" :src="mediaUrl(c.author.avatar_path)" alt="" class="comment-avatar" />
              <span v-else class="comment-avatar comment-avatar-initials">{{ authorInitials(c.author) }}</span>
            </RouterLink>

            <div class="comment-body">
              <div class="comment-head">
                <RouterLink :to="`/profile/${c.user_id}`" class="comment-author">{{ authorName(c.author) }}</RouterLink>
                <time class="comment-time" :datetime="c.created_at">{{ formatTimestamp(c.created_at) }}</time>
                <ConfirmDelete
                  v-if="c.user_id === auth.user?.id"
                  noun="comment"
                  @confirm="deleteComment(post.id, c.id)"
                />
              </div>
              <p class="comment-content">{{ c.content }}</p>
              <img
                v-if="c.image_path"
                :src="mediaUrl(c.image_path)"
                :alt="`Image from ${authorName(c.author)}`"
                class="comment-image"
              />
            </div>
          </li>
        </TransitionGroup>

        <form class="comment-form" @submit.prevent="submitComment">
          <label for="comment-text" class="visually-hidden">Write a comment</label>
          <textarea
            id="comment-text"
            ref="commentInput"
            v-model="newCommentText[post.id]"
            class="text-input"
            rows="2"
            maxlength="1000"
            placeholder="Write a comment..."
          ></textarea>
          <EmojiButton target="comment-text" />
          <div class="comment-form-row">
            <input
              :key="commentFileKey"
              type="file"
              class="file-input"
              accept="image/jpeg,image/png,image/gif"
              aria-label="Attach an image"
              @change="onCommentImagePicked(post.id, $event)"
            />
            <button type="submit" class="comment-submit">Comment</button>
          </div>
        </form>
      </section>
    </template>
  </div>
</template>

<style scoped src="../styles/controls.css"></style>

<style scoped>
.post-detail {
  max-width: 42.5rem;
  margin: 0 auto;
  padding-block: 32px var(--space-7);
}

.back-button {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 2.5rem;
  margin: 0 0 var(--space-4) -14px;
  padding: var(--space-2) 14px;
  background: none;
  border: 0;
  border-radius: var(--radius-pill);
  color: var(--text-muted);
  font-size: var(--text-label);
  font-weight: 700;
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.back-button:hover {
  background-color: var(--badge-purple-bg);
  color: var(--primary);
}

.back-button .nav-icon {
  width: 1.125rem;
  height: 1.125rem;
  transition: transform var(--duration-base) var(--ease-out);
}

.back-button:hover .nav-icon {
  transform: translateX(-3px);
}

.unavailable {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-3);
  padding: 22px 24px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
}

.text-link {
  color: var(--primary);
  font-weight: 700;
  text-decoration: none;
}

.text-link:hover {
  text-decoration: underline;
  text-underline-offset: 0.2em;
}

.action-error {
  margin-bottom: var(--space-3);
}

.comments {
  margin-top: 16px;
  padding: 22px 24px 24px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
}

.comments-title {
  margin: 0 0 var(--space-4);
}

.comment-list {
  position: relative;
  margin: 0;
  padding: 0;
  list-style: none;
}

.comment {
  display: flex;
  gap: var(--space-3);
  padding-block: var(--space-4);
}

.comment:first-child {
  padding-top: 0;
}

.comment + .comment {
  border-top: 1px solid var(--border-hairline);
}

.comment-avatar-link {
  flex: none;
  text-decoration: none;
  border-radius: 50%;
}

.comment-avatar {
  display: block;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 50%;
  object-fit: cover;
}

.comment-avatar-initials {
  display: grid;
  place-items: center;
  background-color: var(--badge-purple-bg);
  color: var(--badge-purple-fg);
  font-size: var(--text-meta);
  font-weight: 700;
}

.comment-body {
  flex: 1;
  min-width: 0;
}

.comment-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: var(--space-1) 10px;
}

.comment-author {
  color: var(--text);
  font-size: var(--text-sm);
  font-weight: 700;
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out);
}

.comment-author:hover {
  color: var(--primary);
}

.comment-time {
  color: var(--text-muted);
  font-size: var(--text-meta);
}

.comment-content {
  margin: 2px 0 0;
  color: var(--text);
  font-size: var(--text-body);
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.comment-image {
  display: block;
  max-width: 100%;
  max-height: 16rem;
  margin-top: var(--space-2);
  background-color: var(--badge-purple-bg);
  border-radius: 14px;
}

.comment-list-enter-active {
  transition:
    opacity 360ms var(--ease-out),
    transform 360ms var(--ease-out);
}

.comment-list-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.comment-list-leave-active {
  position: absolute;
  right: 0;
  left: 0;
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.comment-list-leave-to {
  opacity: 0;
  transform: translateX(12px);
}

.comment-list-move {
  transition: transform 320ms var(--ease-out);
}

.comment-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--border-hairline);
}

.comment-form .text-input {
  min-height: 3.25rem;
  padding: 14px 18px;
  background-color: var(--bg);
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  font-size: var(--text-body);
  line-height: 1.5;
  transition:
    background-color var(--duration-base) var(--ease-out),
    border-color var(--duration-base) var(--ease-out),
    box-shadow var(--duration-base) var(--ease-out);
}

.comment-form .text-input::placeholder {
  color: var(--text-placeholder);
}

.comment-form .text-input:hover {
  border-color: var(--border);
}

.comment-form .text-input:focus-visible {
  background-color: var(--surface);
  border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
  outline: none;
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--primary) 14%, transparent);
}

.comment-form-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}

.comment-form .file-input {
  min-width: 0;
  font-size: var(--text-meta);
}

.comment-form .file-input::file-selector-button {
  padding: 7px 14px;
  background-color: var(--badge-purple-bg);
  border: 0;
  border-radius: var(--radius-pill);
  color: var(--badge-purple-fg);
  font-weight: 700;
  transition: background-color var(--duration-fast) var(--ease-out);
}

.comment-form .file-input::file-selector-button:hover {
  background-color: color-mix(in srgb, var(--primary) 18%, var(--surface));
}

.comment-submit {
  min-height: 2.75rem;
  padding: 0 22px;
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

.comment-submit:hover {
  background-color: var(--primary-hover);
  box-shadow: var(--shadow-primary-lift);
  transform: translateY(-2px);
}

.comment-submit:active {
  box-shadow: var(--shadow-primary);
  transform: translateY(0);
  transition-duration: 80ms;
}

.comment-submit:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 3px;
}

@media (max-width: 47.99rem) {
  .post-detail {
    padding: 16px 16px 40px;
  }

  .comments {
    padding: 18px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .comment-list-enter-active,
  .comment-list-leave-active,
  .comment-list-move,
  .comment-form .text-input,
  .comment-submit,
  .back-button,
  .back-button .nav-icon {
    transition-duration: 1ms;
  }

  .comment-submit:hover,
  .back-button:hover .nav-icon {
    transform: none;
  }
}
</style>
