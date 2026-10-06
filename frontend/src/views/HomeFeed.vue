<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import PostCard from '../components/PostCard.vue'
import UserSearch from '../components/UserSearch.vue'

const router = useRouter()

const posts = ref([])
const loading = ref(true)
const error = ref('')

// Fetches the feed from GET /api/posts.
async function loadFeed() {
  loading.value = true
  error.value = ''
  try {
    posts.value = await api.get('/api/posts')
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

// Deletes one of the user's posts and drops it from the list.
async function deletePost(id) {
  error.value = ''
  try {
    await api.del(`/api/posts/${id}`)
    posts.value = posts.value.filter((p) => p.id !== id)
  } catch (e) {
    error.value = e.message
  }
}

// Swaps in an updated post, e.g. after a like, from a PostCard.
function replacePost(updated) {
  const index = posts.value.findIndex((p) => p.id === updated.id)
  if (index !== -1) {
    posts.value[index] = updated
  }
}

// Navigates to a user's profile, used by the user search box.
function openProfile(user) {
  router.push(`/profile/${user.id}`)
}

onMounted(async () => {
  await loadFeed()

  // vue-router tried to restore this before the posts had loaded, when the page was too short.
  const saved = window.history.state?.scroll
  if (saved) {
    await nextTick()
    window.scrollTo(saved.left, saved.top)
  }
})
</script>

<template>
  <div class="home-feed">
    <h1 class="feed-title">Feed</h1>

    <div class="feed-search">
      <UserSearch label="Search people" @select="openProfile" />
    </div>

    <p v-if="error" class="error feed-message" role="alert">{{ error }}</p>
    <p v-if="loading" class="status feed-message">Loading feed...</p>

    <TransitionGroup
      v-if="posts.length"
      tag="ul"
      name="post-list"
      class="posts"
      @before-leave="(el) => (el.style.top = `${el.offsetTop}px`)"
    >
      <li v-for="(post, index) in posts" :key="post.id" :style="{ '--stagger': Math.min(index, 8) }">
        <PostCard :post="post" mode="feed" @update:post="replacePost" @delete="deletePost" />
      </li>
    </TransitionGroup>
  </div>
</template>

<style scoped src="../styles/controls.css"></style>
<style scoped src="./HomeFeed.css"></style>
