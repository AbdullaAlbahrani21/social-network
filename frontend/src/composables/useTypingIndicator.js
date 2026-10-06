import { computed, toValue } from 'vue'
import { useTypingStore } from '../stores/typing'
import { useWebSocketStore } from '../stores/websocket'

export function useTypingIndicator(type, id) {
  const typingStore = useTypingStore()
  const wsStore = useWebSocketStore()

  const target = () => {
    const t = toValue(type)
    const i = toValue(id)
    return t && i != null ? { t, i } : null
  }

  const typingUserIds = computed(() => {
    const conv = target()
    return conv ? typingStore.typersIn(conv.t, conv.i) : []
  })

  function onInput(text) {
    const conv = target()
    if (!conv || !String(text ?? '').trim()) return
    if (typingStore.readyToSignal(conv.t, conv.i)) {
      wsStore.send({ type: 'typing', targetType: conv.t, targetId: conv.i })
    }
  }

  function onSent() {
    const conv = target()
    if (conv) typingStore.sentMessage(conv.t, conv.i)
  }

  return { typingUserIds, onInput, onSent }
}
