<script setup>
import { computed, ref } from 'vue'
import NavIcon from './NavIcon.vue'

const props = defineProps({
  options: { type: Array, required: true },
  modelValue: { type: [String, Boolean, Number], default: null },
  label: { type: String, required: true },
  disabled: { type: Boolean, default: false },
  name: { type: String, default: null },
  appearance: { type: String, default: 'tinted' },
})

const emit = defineEmits(['select'])

const activeIndex = computed(() => props.options.findIndex((o) => o.value === props.modelValue))

const buttons = ref([])

function focus() {
  const index = Math.max(activeIndex.value, 0)
  const el = buttons.value[index]
  ;(el?.querySelector?.('input') ?? el)?.focus()
}

defineExpose({ focus })
</script>

<template>
  <div
    class="segmented"
    :class="`is-${appearance}`"
    :role="name ? 'radiogroup' : 'group'"
    :aria-label="label"
    :aria-disabled="disabled ? 'true' : undefined"
    :style="{ '--segments': options.length, '--active': Math.max(activeIndex, 0) }"
  >
    <span class="segmented-indicator" :class="{ 'is-hidden': activeIndex < 0 }" aria-hidden="true"></span>

    <template v-if="name">
      <label
        v-for="option in options"
        :key="String(option.value)"
        ref="buttons"
        class="segment"
        :class="{ 'is-active': option.value === modelValue, 'is-disabled': disabled }"
      >
        <input
          type="radio"
          class="visually-hidden"
          :name="name"
          :value="option.value"
          :checked="option.value === modelValue"
          :disabled="disabled"
          @change="emit('select', option.value)"
        />
        <NavIcon v-if="option.icon" :name="option.icon" />
        <span>{{ option.label }}</span>
      </label>
    </template>

    <template v-else>
      <button
        v-for="option in options"
        :key="String(option.value)"
        ref="buttons"
        type="button"
        class="segment"
        :class="{ 'is-active': option.value === modelValue }"
        :aria-pressed="option.value === modelValue ? 'true' : 'false'"
        :disabled="disabled"
        @click="emit('select', option.value)"
      >
        <NavIcon v-if="option.icon" :name="option.icon" />
        <span>{{ option.label }}</span>
      </button>
    </template>
  </div>
</template>

<style scoped>
.segmented {
  position: relative;
  display: inline-grid;
  grid-template-columns: repeat(var(--segments), minmax(0, 1fr));
  padding: 4px;
  background-color: var(--badge-purple-bg);
  border-radius: var(--radius-pill);
  isolation: isolate;
}

.segmented.is-outline {
  background-color: var(--surface);
  border: 1px solid var(--border);
}

.segmented-indicator {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 4px;
  z-index: -1;
  width: calc((100% - 8px) / var(--segments));
  background-color: var(--surface);
  border-radius: var(--radius-pill);
  box-shadow: 0 6px 14px -8px color-mix(in srgb, var(--primary) 55%, transparent);
  transform: translateX(calc(var(--active) * 100%));
  transition:
    transform var(--duration-base) var(--ease-out),
    opacity var(--duration-fast) var(--ease-out);
}

.is-outline .segmented-indicator {
  background-color: var(--badge-purple-bg);
  box-shadow: none;
}

.segmented-indicator.is-hidden {
  opacity: 0;
}

.segment {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 0;
  min-height: 2.25rem;
  padding: 0 18px;
  background: none;
  border: 0;
  border-radius: var(--radius-pill);
  color: var(--text-muted);
  font-size: var(--text-label);
  font-weight: 700;
  white-space: nowrap;
  cursor: pointer;
  transition: color var(--duration-base) var(--ease-out);
}

.is-outline .segment {
  min-height: 2.5rem;
}

.segment:hover:not(.is-active):not(:disabled):not(.is-disabled) {
  color: var(--text);
}

.segment.is-active {
  color: var(--primary);
  cursor: default;
}

.segment:focus-visible,
.segment:has(input:focus-visible) {
  outline: 2px solid var(--primary);
  outline-offset: 1px;
}

.segment:disabled,
.segment.is-disabled {
  cursor: progress;
}

.segment .nav-icon {
  flex: none;
  width: 0.9rem;
  height: 0.9rem;
  color: var(--text-placeholder);
  transition: color var(--duration-base) var(--ease-out);
}

.segment.is-active .nav-icon {
  color: var(--primary);
}

.segment span {
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (prefers-reduced-motion: reduce) {
  .segmented-indicator,
  .segment,
  .segment .nav-icon {
    transition-duration: 1ms;
  }
}
</style>
