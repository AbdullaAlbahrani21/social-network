<template>
  <span class="emoji-button">
    <button
      ref="toggle"
      type="button"
      class="emoji-toggle"
      aria-label="Add emoji"
      aria-haspopup="true"
      :aria-expanded="open"
      @click="open ? close() : show()"
    >
      <span aria-hidden="true">🙂</span>
    </button>

    <!-- Teleported and fixed so a scrolling or overflow-clipped parent cannot cut it off. -->
    <Teleport to="body">
      <div
        v-if="open"
        ref="popover"
        class="emoji-popover"
        role="dialog"
        aria-label="Emoji"
        :style="position"
      >
        <button
          v-for="emoji in emojis"
          :key="emoji"
          type="button"
          class="emoji-option"
          :aria-label="emoji"
          @click="pick(emoji)"
        >
          {{ emoji }}
        </button>
      </div>
    </Teleport>
  </span>
</template>

<script setup>
import { nextTick, onBeforeUnmount, ref } from 'vue'

// An element, or the id of one, resolved at pick time so the field only has to exist by then.
const props = defineProps({
  target: { type: [String, Object], required: true },
})

const emojis = [
  '😀', '😃', '😄', '😁', '😆', '😅', '😂', '🤣',
  '😊', '😇', '🙂', '😉', '😍', '🥰', '😘', '😋',
  '😎', '🤩', '🥳', '🤔', '😮', '😢', '😭', '😡',
  '❤️', '🧡', '💛', '💚', '💙', '💜', '👍', '👎',
  '👏', '🙌', '🙏', '💪', '🔥', '🎉', '✨', '💯',
]

const GAP = 8
const EDGE = 8

const open = ref(false)
const toggle = ref(null)
const popover = ref(null)
const position = ref({})

function field() {
  return typeof props.target === 'string' ? document.getElementById(props.target) : props.target
}

async function show() {
  open.value = true
  position.value = { visibility: 'hidden' }
  await nextTick()
  place()

  document.addEventListener('pointerdown', onOutside, true)
  document.addEventListener('keydown', onKeydown, true)
  window.addEventListener('resize', place)
  window.addEventListener('scroll', place, true)
  popover.value.querySelector('button')?.focus()
}

// Above the button when there is room, else below, clamped to the viewport; re-run on scroll and resize (a phone keyboard closing is a resize).
function place() {
  if (!popover.value || !toggle.value) return
  const button = toggle.value.getBoundingClientRect()
  const { width, height } = popover.value.getBoundingClientRect()
  const viewportWidth = document.documentElement.clientWidth

  const left = Math.min(Math.max(button.right - width, EDGE), viewportWidth - width - EDGE)
  const above = button.top - height - GAP
  const top = above >= EDGE ? above : button.bottom + GAP
  position.value = { left: `${left}px`, top: `${top}px` }
}

function close(refocus = false) {
  open.value = false
  document.removeEventListener('pointerdown', onOutside, true)
  document.removeEventListener('keydown', onKeydown, true)
  window.removeEventListener('resize', place)
  window.removeEventListener('scroll', place, true)
  if (refocus) toggle.value?.focus()
}

function onOutside(event) {
  if (popover.value?.contains(event.target) || toggle.value?.contains(event.target)) return
  close()
}

// Capturing, so an Escape meant for the picker does not also reach a surrounding popover or dialog.
function onKeydown(event) {
  if (event.key !== 'Escape') return
  event.preventDefault()
  event.stopPropagation()
  close(true)
}

function pick(emoji) {
  const el = field()
  if (!el || el.disabled || el.readOnly) return

  const value = el.value
  const start = el.selectionStart ?? value.length
  const end = el.selectionEnd ?? value.length

  // maxLength counts UTF-16 units like emoji.length does, and a programmatic edit is not held to it, so check here.
  if (el.maxLength >= 0 && value.length - (end - start) + emoji.length > el.maxLength) return

  el.value = value.slice(0, start) + emoji + value.slice(end)
  const caret = start + emoji.length
  el.setSelectionRange(caret, caret)
  el.dispatchEvent(new Event('input', { bubbles: true }))

  close()
  el.focus()
}

onBeforeUnmount(close)
</script>

<style scoped>
.emoji-button {
  display: inline-flex;
  flex: none;
}

.emoji-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  padding: 0;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-pill);
  font-size: var(--text-lg);
  line-height: 1;
  cursor: pointer;
  transition: border-color var(--duration-fast) var(--ease-out);
}

.emoji-toggle:hover {
  border-color: var(--border);
}

.emoji-toggle:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.emoji-popover {
  position: fixed;
  z-index: 1000;
  display: grid;
  grid-template-columns: repeat(8, 2.25rem);
  gap: 2px;
  max-width: calc(100vw - 16px);
  padding: var(--space-2);
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
}

.emoji-option {
  width: 2.25rem;
  height: 2.25rem;
  padding: 0;
  background: none;
  border: 0;
  border-radius: var(--radius-sm);
  font-size: 1.25rem;
  line-height: 1;
  cursor: pointer;
}

.emoji-option:hover,
.emoji-option:focus-visible {
  background-color: var(--bg);
  outline: none;
}

.emoji-option:focus-visible {
  box-shadow: 0 0 0 2px var(--primary);
}
</style>
