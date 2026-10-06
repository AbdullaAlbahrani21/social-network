import { defineStore } from 'pinia'
import { ref } from 'vue'
import { keyFor } from './chat'

// Longer than SIGNAL_INTERVAL_MS, which is what makes a missed repeat read as having stopped.
const SHOW_FOR_MS = 3000

const SIGNAL_INTERVAL_MS = 2000

export const useTypingStore = defineStore('typing', () => {
  const typing = ref({})
  const expiries = new Map()
  const lastSignalled = new Map()

  function typersIn(type, id) {
    return typing.value[keyFor(type, id)] || []
  }

  function stopShowing(key, userId) {
    const timerKey = `${key}|${userId}`
    clearTimeout(expiries.get(timerKey))
    expiries.delete(timerKey)

    const list = typing.value[key]
    if (!list) return
    const rest = list.filter((id) => id !== userId)
    if (rest.length) typing.value[key] = rest
    else delete typing.value[key]
  }

  function receiveTyping(event, currentUserID) {
    const from = event?.fromUserId
    if (!from || from === currentUserID) return

    let key
    if (event.targetType === 'group') key = keyFor('group', event.targetId)
    else if (event.targetType === 'dm' && event.targetId === currentUserID) key = keyFor('dm', from)
    else return

    const list = typing.value[key] || []
    if (!list.includes(from)) typing.value[key] = [...list, from]

    const timerKey = `${key}|${from}`
    clearTimeout(expiries.get(timerKey))
    expiries.set(timerKey, setTimeout(() => stopShowing(key, from), SHOW_FOR_MS))
  }

  function messageArrived(msg, currentUserID) {
    if (!msg?.sender_id || msg.sender_id === currentUserID) return
    const key = msg.group_id ? keyFor('group', msg.group_id) : keyFor('dm', msg.sender_id)
    stopShowing(key, msg.sender_id)
  }

  function readyToSignal(type, id) {
    const key = keyFor(type, id)
    const now = Date.now()
    if (now - (lastSignalled.get(key) ?? -Infinity) < SIGNAL_INTERVAL_MS) return false
    lastSignalled.set(key, now)
    return true
  }

  function sentMessage(type, id) {
    lastSignalled.delete(keyFor(type, id))
  }

  function reset() {
    for (const timer of expiries.values()) clearTimeout(timer)
    expiries.clear()
    lastSignalled.clear()
    typing.value = {}
  }

  return { typing, typersIn, receiveTyping, messageArrived, readyToSignal, sentMessage, reset }
})
