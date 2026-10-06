import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api } from '../api'

// A user and a group can share a numeric id -- user 9 and group 9 are unrelated -- so a bare id cannot tell a DM from a group channel.
export function keyFor(type, id) {
  if (type !== 'dm' && type !== 'group') {
    throw new Error(`chat: unknown conversation type ${JSON.stringify(type)}`)
  }
  return `${type}:${id}`
}

// No safe default: the old 0 matched nobody, so the viewer's own messages were read as someone else's.
function requireUserID(currentUserID, caller) {
  if (!currentUserID) {
    throw new Error(`chat: ${caller} needs the current user's id`)
  }
}

export const useChatStore = defineStore('chat', () => {
  const conversations = ref({})
  const activeChatID = ref(null)
  const activeChatType = ref('dm')

  // A live message for a chat that was never opened creates the key too, so whether a key exists cannot answer this.
  const historyRequested = new Set()

  // A request that started before a reset belongs to the previous user, so its result is dropped.
  let generation = 0

  const activeMessages = computed(() => {
    if (activeChatID.value == null) return []
    return conversations.value[keyFor(activeChatType.value, activeChatID.value)] || []
  })

  function listFor(key) {
    if (!conversations.value[key]) {
      conversations.value[key] = []
    }
    // Read back through the reactive object: the plain array assigned above is not the proxy, and pushing to it would not update the view.
    return conversations.value[key]
  }

  function setActiveChat(id, type = 'dm') {
    activeChatID.value = id
    activeChatType.value = type
    if (!historyRequested.has(keyFor(type, id))) {
      fetchConversation(id, type)
    }
  }

  async function fetchConversation(targetID, type) {
    const key = keyFor(type, targetID)
    const endpoint = type === 'group'
      ? `/api/groups/${targetID}/messages`
      : `/api/messages/${targetID}`
    const startedIn = generation

    historyRequested.add(key)

    try {
      const data = await api.get(endpoint)
      if (startedIn !== generation) return

      // The fetched rows come first, then whatever they don't already contain: a live message or the viewer's own send may have arrived mid-request.
      const history = (data || []).reverse()
      const fetchedIDs = new Set(history.map((m) => m.id))
      const arrived = (conversations.value[key] || []).filter(
        (m) => m.id == null || !fetchedIDs.has(m.id),
      )
      conversations.value[key] = [...history, ...arrived]
    } catch (err) {
      if (startedIn === generation) historyRequested.delete(key)
      console.error('Error fetching chat history:', err)
    }
  }

  // Every send is delivered twice, by the HTTP reply and by the WebSocket echo, in either order.
  function fileMessage(key, msg, tempID) {
    const list = listFor(key)

    if (tempID) {
      const idx = list.findIndex((m) => m.temp_id === tempID)
      if (idx !== -1) {
        list[idx] = { ...msg, pending: false }
        return
      }
    }

    if (msg.id != null && list.some((m) => m.id === msg.id)) return
    list.push(msg)
  }

  function markFailed(key, tempID) {
    const list = conversations.value[key] || []
    const idx = list.findIndex((m) => m.temp_id === tempID && m.pending)
    if (idx !== -1) {
      list[idx] = { ...list[idx], pending: false, failed: true }
    }
  }

  async function sendMessage(targetID, content, type, currentUserID) {
    requireUserID(currentUserID, 'sendMessage')

    const key = keyFor(type, targetID)
    const isGroup = type === 'group'
    const tempID = `temp_${Date.now()}`
    const startedIn = generation

    listFor(key).push({
      id: null,
      temp_id: tempID,
      sender_id: currentUserID,
      receiver_id: isGroup ? 0 : targetID,
      group_id: isGroup ? targetID : 0,
      content: content,
      created_at: new Date().toISOString(),
      pending: true
    })

    const endpoint = isGroup ? '/api/groups/messages' : '/api/messages'
    const bodyPayload = isGroup
      ? { group_id: targetID, content, temp_id: tempID }
      : { to_id: targetID, content, temp_id: tempID }

    try {
      const savedMsg = await api.post(endpoint, bodyPayload)
      if (startedIn !== generation) return
      fileMessage(key, savedMsg, tempID)
    } catch (err) {
      console.error('Failed to send message:', err)
      if (startedIn !== generation) return
      markFailed(key, tempID)
    }
  }

  function receiveMessage(msg, tempID, currentUserID) {
    requireUserID(currentUserID, 'receiveMessage')

    let key
    if (msg.group_id) {
      key = keyFor('group', msg.group_id)
    } else if (msg.sender_id === currentUserID) {
      key = keyFor('dm', msg.receiver_id)
    } else {
      key = keyFor('dm', msg.sender_id)
    }

    // The server echoes the sender's temp_id to the recipient too, so matching on it for someone else's message could replace an unrelated pending send.
    const ownTempID = msg.sender_id === currentUserID ? tempID : null

    fileMessage(key, msg, ownTempID)
  }

  function reset() {
    generation += 1
    historyRequested.clear()
    conversations.value = {}
    activeChatID.value = null
    activeChatType.value = 'dm'
  }

  return {
    conversations,
    activeChatID,
    activeChatType,
    activeMessages,
    setActiveChat,
    fetchConversation,
    sendMessage,
    receiveMessage,
    reset
  }
})
