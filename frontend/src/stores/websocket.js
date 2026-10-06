import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useChatStore } from './chat'
import { useNotificationStore } from './notifications'
import { useTypingStore } from './typing'
import { BASE_URL } from '../api'

// Exponential with jitter, so every client does not retry in lockstep after a server restart.
const RECONNECT_BASE_MS = 1000
const RECONNECT_MAX_MS = 30000

const CLOSE_SESSION_ENDED = 1008

// These strings are a wire contract with pkg/auth and pkg/websocket, matched exactly.
export const CLOSE_REASON_LOGGED_OUT = 'logged out'
export const CLOSE_REASON_LOGGED_IN_ELSEWHERE = 'logged in elsewhere'

export const useWebSocketStore = defineStore('websocket', () => {
  const socket = ref(null)
  const isConnected = ref(false)
  // App.vue owns what happens next: the router imports the auth store, so a store importing the router back would be circular.
  const sessionEnded = ref(null)

  let reconnectAttempts = 0
  let reconnectTimer = null
  let stopped = false
  let connectedUserID = null
  // Views that want a live feed of one frame type subscribe here; the store itself only knows about the stores it owns.
  const listeners = new Map()
  let generation = 0

  function reconnectDelay() {
    const backoff = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempts, RECONNECT_MAX_MS)
    reconnectAttempts += 1
    return backoff + Math.random() * 1000
  }

  // The socket cannot say why it closed: a rejected handshake and a dropped connection both surface as 1006, and the 401 never reaches JS.
  async function sessionOwner() {
    try {
      const res = await fetch(`${BASE_URL}/api/me`, { credentials: 'include' })
      if (res.status === 401) return 'ended'
      if (!res.ok) return 'unknown'
      const me = await res.json()
      return me.id === connectedUserID ? 'same' : 'other'
    } catch {
      return 'unknown'
    }
  }

  function scheduleReconnect() {
    if (stopped || reconnectTimer) return

    const checkedFor = generation
    sessionOwner().then((owner) => {
      if (checkedFor !== generation || stopped || reconnectTimer) return

      if (owner === 'ended' || owner === 'other') {
        stopped = true
        console.warn(
          owner === 'ended'
            ? 'WebSocket: session rejected, not reconnecting'
            : 'WebSocket: a different user is logged in, not reconnecting',
        )
        return
      }

      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        connect(connectedUserID)
      }, reconnectDelay())
    })
  }

  function connect(userID) {
    if (socket.value && userID !== connectedUserID) {
      disconnect()
    }

    // CONNECTING counts as live: without this a reconnect firing alongside App.vue's watcher would open a second socket.
    if (
      socket.value &&
      (socket.value.readyState === WebSocket.OPEN || socket.value.readyState === WebSocket.CONNECTING)
    ) {
      return
    }

    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    stopped = false
    sessionEnded.value = null
    connectedUserID = userID

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws`

    const ws = new WebSocket(wsUrl)
    socket.value = ws

    ws.onopen = () => {
      isConnected.value = true
      reconnectAttempts = 0
      console.log('WebSocket connected')
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        handleIncomingMessage(data)
      } catch (err) {
        console.error('Failed to parse WS payload:', err)
      }
    }

    // disconnect() detaches these handlers before closing, so a close arriving here was never this tab's own doing.
    ws.onclose = (event) => {
      isConnected.value = false
      if (event.code === CLOSE_SESSION_ENDED) {
        stopped = true
        sessionEnded.value =
          event.reason === CLOSE_REASON_LOGGED_IN_ELSEWHERE
            ? CLOSE_REASON_LOGGED_IN_ELSEWHERE
            : CLOSE_REASON_LOGGED_OUT
        return
      }
      scheduleReconnect()
    }

    ws.onerror = (err) => {
      console.error('WebSocket error:', err)
    }
  }

  function disconnect() {
    generation += 1
    stopped = true
    connectedUserID = null
    reconnectAttempts = 0
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }

    const old = socket.value
    socket.value = null
    isConnected.value = false

    if (old) {
      old.onopen = null
      old.onmessage = null
      old.onclose = null
      old.onerror = null
      old.close()
    }
  }

  // Returns the unsubscribe: a view that forgets to call it on unmount would keep updating a list nobody is looking at.
  function on(type, handler) {
    if (!listeners.has(type)) listeners.set(type, new Set())
    listeners.get(type).add(handler)

    return () => {
      const subscribed = listeners.get(type)
      if (!subscribed) return

      subscribed.delete(handler)
      if (subscribed.size === 0) listeners.delete(type)
    }
  }

  // A copy, so a handler that unsubscribes itself does not mutate the set being walked.
  function notifyListeners(data) {
    const subscribed = listeners.get(data.type)
    if (!subscribed) return false

    for (const handler of [...subscribed]) {
      try {
        handler(data)
      } catch (err) {
        console.error(`WS listener for "${data.type}" failed:`, err)
      }
    }

    return true
  }

  function handleIncomingMessage(data) {
    const chatStore = useChatStore()
    const notifStore = useNotificationStore()
    const typingStore = useTypingStore()

    const delivered = notifyListeners(data)

    switch (data.type) {
      case 'message':
      case 'group_message':
        chatStore.receiveMessage(data.message, data.temp_id, connectedUserID)
        typingStore.messageArrived(data.message, connectedUserID)
        break
      case 'typing':
        typingStore.receiveTyping(data, connectedUserID)
        break
      case 'notification':
        notifStore.receiveNotification(data.notification, data.is_new !== false)
        break
      default:
        if (!delivered) console.log('Unhandled WS message type:', data.type)
    }

  }

  function send(payload) {
    if (socket.value && socket.value.readyState === WebSocket.OPEN) {
      socket.value.send(JSON.stringify(payload))
    }
  }

  return { socket, isConnected, sessionEnded, connect, disconnect, send, on }
})
