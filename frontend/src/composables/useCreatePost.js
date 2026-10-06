import { ref } from 'vue'
import { api } from '../api'

// Shared state and action for creating a post.
export function useCreatePost() {
  const posting = ref(false)
  const error = ref('')

  // Builds the multipart form (text, privacy, audience, image) and sends it to POST /api/posts.
  async function createPost({ content, privacy = 'public', visibilityUserIds = [], image = null }) {
    if (!content.trim()) return null
    posting.value = true
    error.value = ''
    try {
      const formData = new FormData()
      formData.append('content', content)
      formData.append('privacy', privacy)
      if (privacy === 'private') {
        for (const id of visibilityUserIds) {
          formData.append('visibility_user_ids', id)
        }
      }
      if (image) {
        formData.append('image', image)
      }
      return await api.upload('/api/posts', formData)
    } catch (e) {
      error.value = e.message
      return null
    } finally {
      posting.value = false
    }
  }

  return { posting, error, createPost }
}
