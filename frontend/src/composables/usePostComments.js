import { ref } from 'vue'
import { api } from '../api'

// Shared state and actions for a post's comments.
export function usePostComments() {
  const commentsByPost = ref({})
  const newCommentText = ref({})
  const newCommentImage = ref({})
  const error = ref('')

  // Fetches a post's comments.
  async function loadComments(postId) {
    error.value = ''
    try {
      commentsByPost.value[postId] = await api.get(`/api/posts/${postId}/comments`)
    } catch (e) {
      error.value = e.message
    }
  }

  // Remembers the image chosen for a post's next comment.
  function onCommentImagePicked(postId, e) {
    newCommentImage.value[postId] = e.target.files[0] || null
  }

  // Deletes one of the user's comments and drops it from the list.
  async function deleteComment(postId, commentId) {
    error.value = ''
    try {
      await api.del(`/api/posts/${postId}/comments/${commentId}`)
      commentsByPost.value[postId] = commentsByPost.value[postId].filter((c) => c.id !== commentId)
    } catch (e) {
      error.value = e.message
    }
  }

  // Sends a new comment (with optional image), then reloads the comments.
  async function addComment(postId) {
    const text = (newCommentText.value[postId] || '').trim()
    if (!text) return
    error.value = ''
    try {
      const formData = new FormData()
      formData.append('content', text)
      if (newCommentImage.value[postId]) {
        formData.append('image', newCommentImage.value[postId])
      }
      await api.upload(`/api/posts/${postId}/comments`, formData)
      newCommentText.value[postId] = ''
      newCommentImage.value[postId] = null
      commentsByPost.value[postId] = await api.get(`/api/posts/${postId}/comments`)
    } catch (e) {
      error.value = e.message
    }
  }

  return {
    commentsByPost,
    newCommentText,
    newCommentImage,
    error,
    loadComments,
    onCommentImagePicked,
    deleteComment,
    addComment,
  }
}
