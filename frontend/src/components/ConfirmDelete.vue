<script setup>
import { nextTick, ref } from 'vue'

defineProps({
  noun: { type: String, required: true },
})

const emit = defineEmits(['confirm'])

const confirming = ref(false)
const trigger = ref(null)
const cancelButton = ref(null)

async function ask() {
  confirming.value = true
  await nextTick()
  cancelButton.value?.focus()
}

async function cancel() {
  confirming.value = false
  await nextTick()
  trigger.value?.focus()
}

function confirm() {
  confirming.value = false
  emit('confirm')
}
</script>

<template>
  <!-- Clicks stay here: in the feed, a click elsewhere on a card opens the post. -->
  <span class="confirm-delete" @click.stop>
    <!-- Default (simultaneous) mode, not out-in: the incoming state is in the DOM at once, so ask() and cancel() can move focus to it on the next tick. -->
    <Transition name="confirm-swap">
      <span
        v-if="confirming"
        class="confirm"
        role="group"
        :aria-label="`Delete this ${noun}?`"
        @keydown.esc="cancel"
      >
        <span class="question" aria-hidden="true">Delete?</span>
        <button type="button" class="yes" @click="confirm">Yes</button>
        <button ref="cancelButton" type="button" class="cancel" @click="cancel">Cancel</button>
      </span>

      <button v-else ref="trigger" type="button" class="trigger" @click="ask">Delete</button>
    </Transition>
  </span>
</template>

<style scoped>
.confirm-delete {
  position: relative;
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: flex-end;
  margin-left: auto;
}

.trigger {
  padding: var(--space-1) 10px;
  background: none;
  border: 0;
  border-radius: var(--radius-pill);
  color: var(--text-placeholder);
  font-size: var(--text-meta);
  font-weight: 600;
  cursor: pointer;
  transition:
    color var(--duration-fast) var(--ease-out),
    background-color var(--duration-fast) var(--ease-out);
}

.trigger:hover {
  background-color: var(--danger-wash);
  color: var(--danger);
}

.confirm {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: 3px 3px 3px 12px;
  background-color: var(--danger-wash);
  border: 1px solid var(--danger-border);
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.question {
  margin-right: var(--space-1);
  color: var(--danger-text-on-wash);
  font-size: var(--text-meta);
  font-weight: 700;
}

.yes,
.cancel {
  min-height: 1.875rem;
  padding: 0 var(--space-3);
  border-radius: var(--radius-pill);
  font-size: var(--text-meta);
  cursor: pointer;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    box-shadow var(--duration-fast) var(--ease-out);
}

.yes {
  background-color: var(--danger);
  border: 0;
  color: var(--surface);
  font-weight: 700;
}

.yes:hover {
  background-color: var(--danger-hover);
  box-shadow: 0 6px 14px -6px color-mix(in srgb, var(--danger) 70%, transparent);
}

.cancel {
  background: none;
  border: 0;
  color: var(--danger-text-on-wash);
  font-weight: 600;
}

.cancel:hover {
  background-color: var(--danger-wash-alt);
  color: var(--text);
}

.yes:focus-visible,
.cancel:focus-visible {
  outline-color: var(--danger);
  outline-offset: 1px;
}

.confirm-swap-enter-active,
.confirm-swap-leave-active {
  transform-origin: right center;
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.confirm-swap-leave-active {
  position: absolute;
  right: 0;
}

.confirm-swap-enter-from,
.confirm-swap-leave-to {
  opacity: 0;
  transform: scale(0.85);
}

@media (prefers-reduced-motion: reduce) {
  .confirm-swap-enter-active,
  .confirm-swap-leave-active,
  .trigger,
  .yes,
  .cancel {
    transition-duration: 1ms;
  }
}
</style>
