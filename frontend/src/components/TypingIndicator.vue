<script setup>
import { computed } from 'vue'

const props = defineProps({
  names: { type: Array, required: true },
})

const label = computed(() => {
  const names = props.names
  if (names.length === 1) return `${names[0]} is typing…`
  if (names.length === 2) return `${names[0]} and ${names[1]} are typing…`
  return 'Several people are typing…'
})
</script>

<template>
  <div class="typing-slot" aria-live="polite">
    <Transition name="typing">
      <div v-if="names.length" class="typing-indicator">
        <span class="dots" aria-hidden="true"><span></span><span></span><span></span></span>
        <span class="label">{{ label }}</span>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.typing-indicator {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dots {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 2.5rem;
  padding: 0 16px;
  background-color: var(--surface);
  border: 1px solid var(--border-hairline);
  border-radius: 20px 20px 20px 6px;
  box-shadow: 0 6px 16px -12px color-mix(in srgb, var(--primary) 40%, transparent);
}

.dots span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background-color: var(--primary);
  animation: typing-bounce 1.2s var(--ease-in-out) infinite;
}

.dots span:nth-child(2) {
  animation-delay: 0.15s;
}

.dots span:nth-child(3) {
  animation-delay: 0.3s;
}

.label {
  color: var(--text-muted);
  font-size: var(--text-meta);
  font-weight: 600;
}

@keyframes typing-bounce {
  0%,
  60%,
  100% {
    opacity: 0.35;
    transform: translateY(0);
  }
  30% {
    opacity: 1;
    transform: translateY(-5px);
  }
}

@keyframes typing-fade {
  0%,
  100% {
    opacity: 0.35;
  }
  50% {
    opacity: 1;
  }
}

.typing-enter-active,
.typing-leave-active {
  overflow: hidden;
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out),
    max-height var(--duration-base) var(--ease-out),
    padding-bottom var(--duration-base) var(--ease-out);
  max-height: 4rem;
}

.typing-enter-from,
.typing-leave-to {
  max-height: 0;
  padding-bottom: 0 !important;
  opacity: 0;
  transform: translateY(8px);
}

@media (prefers-reduced-motion: reduce) {
  .dots span {
    animation-name: typing-fade;
  }

  .typing-enter-active,
  .typing-leave-active {
    transition-duration: 1ms;
  }
}
</style>
