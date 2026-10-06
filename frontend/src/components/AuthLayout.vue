<script setup>
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

defineProps({
  view: { type: Object, default: null },
})

const route = useRoute()

const activeIndex = computed(() => (route.name === 'register' ? 1 : 0))

const stage = ref(null)
const swapping = ref(false)

function onBeforeLeave() {
  if (!stage.value) return
  stage.value.style.height = `${stage.value.offsetHeight}px`
  swapping.value = true
}

function onEnter(el) {
  if (!stage.value) return
  // Reading the height also flushes the pinned value, so the change below animates from it rather than jumping.
  const height = el.offsetHeight
  stage.value.style.height = `${height}px`
}

function onAfterEnter() {
  if (!stage.value) return
  stage.value.style.height = ''
  swapping.value = false
}
</script>

<template>
  <div class="auth-screen">
    <div class="auth-shell">
      <aside class="auth-panel">
        <span class="blob blob-1" aria-hidden="true"></span>
        <span class="blob blob-2" aria-hidden="true"></span>
        <span class="blob blob-3" aria-hidden="true"></span>

        <nav class="auth-tabs" aria-label="Account">
          <span
            class="auth-tab-indicator"
            :style="{ '--tab-index': activeIndex }"
            aria-hidden="true"
          ></span>

          <RouterLink
            to="/login"
            class="auth-tab"
            :class="{ 'is-active': activeIndex === 0 }"
            :aria-current="activeIndex === 0 ? 'page' : undefined"
          >
            Login
          </RouterLink>
          <RouterLink
            to="/register"
            class="auth-tab"
            :class="{ 'is-active': activeIndex === 1 }"
            :aria-current="activeIndex === 1 ? 'page' : undefined"
          >
            Sign up
          </RouterLink>
        </nav>
      </aside>

      <main class="auth-content">
        <div class="auth-column">
          <div class="auth-brand">
            <span class="logo-mark" aria-hidden="true"></span>
            <p class="wordmark">Social Network</p>
          </div>

          <div
            ref="stage"
            class="auth-stage"
            :class="{ 'is-swapping': swapping }"
          >
            <Transition
              name="auth-swap"
              @before-leave="onBeforeLeave"
              @enter="onEnter"
              @after-enter="onAfterEnter"
            >
              <component :is="view" />
            </Transition>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.auth-screen {
  --tab-width: 124px;
  --tab-height: 100px;
  --tab-radius: var(--radius-md);

  display: flex;
  min-height: 100vh;
  min-height: 100dvh;
  padding: 20px;
  background:
    radial-gradient(60% 40% at 15% 100%, color-mix(in srgb, var(--badge-pink-fg) 9%, transparent), transparent 70%),
    radial-gradient(50% 40% at 90% 100%, color-mix(in srgb, var(--primary) 10%, transparent), transparent 70%),
    var(--bg);
}

.auth-shell {
  display: flex;
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  background-color: var(--surface);
  border-radius: var(--radius-lg);
  box-shadow: 0 30px 60px -30px color-mix(in srgb, var(--primary) 35%, transparent);
  overflow: hidden;
}

.auth-panel {
  position: relative;
  flex: 0 0 34%;
  background: var(--panel-gradient);
  overflow: hidden;
}

.blob {
  position: absolute;
  pointer-events: none;
}

.blob-1 {
  left: -30%;
  bottom: -18%;
  width: 95%;
  height: 105%;
  background-color: rgb(255 255 255 / 0.1);
  border-radius: 48% 52% 40% 60% / 38% 44% 56% 62%;
  transform: rotate(-8deg);
}

.blob-2 {
  left: 12%;
  top: 18%;
  width: 80%;
  height: 92%;
  background-color: color-mix(in srgb, var(--notification-dot) 16%, transparent);
  border-radius: 58% 42% 50% 50% / 30% 40% 60% 70%;
  transform: rotate(10deg);
}

.blob-3 {
  right: -35%;
  top: -22%;
  width: 110%;
  height: 60%;
  background-color: rgb(255 255 255 / 0.08);
  border-radius: 50% 50% 45% 55% / 55% 45% 55% 45%;
}

.auth-tabs {
  position: absolute;
  right: 0;
  /* A fixed offset: the panel grows with the taller register form, and the tab shouldn't drift down with it. */
  top: 320px;
  z-index: 1;
  display: flex;
  flex-direction: column;
  width: var(--tab-width);
}

.auth-tab-indicator {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: var(--tab-height);
  background-color: var(--surface);
  border-radius: var(--tab-radius) 0 0 var(--tab-radius);
  transform: translateY(calc(var(--tab-index) * 100%));
  transition: transform var(--duration-base) var(--ease-out);
}

.auth-tab-indicator::before,
.auth-tab-indicator::after {
  content: "";
  position: absolute;
  right: 0;
  width: var(--tab-radius);
  height: var(--tab-radius);
}

.auth-tab-indicator::before {
  bottom: 100%;
  background: radial-gradient(
    circle at 0 0,
    transparent calc(var(--tab-radius) - 0.5px),
    var(--surface) var(--tab-radius)
  );
}

.auth-tab-indicator::after {
  top: 100%;
  background: radial-gradient(
    circle at 0 100%,
    transparent calc(var(--tab-radius) - 0.5px),
    var(--surface) var(--tab-radius)
  );
}

.auth-tab {
  position: relative;
  display: flex;
  align-items: center;
  height: var(--tab-height);
  padding-left: 24px;
  color: rgb(255 255 255 / 0.92);
  font-size: 1.125rem;
  font-weight: 600;
  text-decoration: none;
  border-radius: var(--tab-radius) 0 0 var(--tab-radius);
  transition:
    color var(--duration-base) var(--ease-out),
    transform var(--duration-fast) var(--ease-out);
}

.auth-tab:hover:not(.is-active) {
  transform: translateX(-3px);
}

.auth-tab.is-active {
  color: color-mix(in srgb, var(--primary) 70%, var(--text));
}

.auth-tab:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: -6px;
  border-radius: var(--radius-sm) 0 0 var(--radius-sm);
}

.auth-content {
  flex: 1 1 auto;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 56px 40px;
}

.auth-column {
  width: 100%;
  max-width: 304px;
}

.auth-brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
  margin-bottom: 40px;
}

.logo-mark {
  width: 136px;
  height: 144px;
  background: var(--logo-gradient);
  -webkit-mask: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 136 144'%3E%3Cpath d='M58 8h56L86 52h38L58 136l14-58H14Z' fill='black' stroke='black' stroke-width='14' stroke-linejoin='round'/%3E%3C/svg%3E") center / contain no-repeat;
  mask: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 136 144'%3E%3Cpath d='M58 8h56L86 52h38L58 136l14-58H14Z' fill='black' stroke='black' stroke-width='14' stroke-linejoin='round'/%3E%3C/svg%3E") center / contain no-repeat;
  filter: drop-shadow(0 12px 18px color-mix(in srgb, var(--primary) 35%, transparent));
}

.wordmark {
  margin: 0;
  font-family: var(--font-display);
  font-size: var(--text-wordmark);
  font-weight: 800;
  line-height: 44px;
  letter-spacing: -0.01em;
  white-space: nowrap;
  background: var(--wordmark-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.auth-stage {
  position: relative;
  transition: height var(--duration-base) var(--ease-out);
}

/* Only while the forms swap: the pinned height would otherwise let the taller form spill over. */
.auth-stage.is-swapping {
  overflow: hidden;
}

.auth-swap-enter-active,
.auth-swap-leave-active {
  transition:
    opacity var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.auth-swap-leave-active {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
}

.auth-swap-enter-from {
  opacity: 0;
  transform: translateY(12px);
}

.auth-swap-leave-to {
  opacity: 0;
  transform: translateY(-12px);
}

@media (max-width: 47.99rem) {
  .auth-screen {
    --tab-width: 50%;
    --tab-height: 56px;
    --tab-radius: 18px;

    padding: 0;
  }

  .auth-shell {
    flex-direction: column;
    border-radius: 0;
    box-shadow: none;
  }

  .auth-panel {
    flex: 0 0 176px;
  }

  .auth-tabs {
    top: auto;
    bottom: 0;
    left: var(--tab-radius);
    right: var(--tab-radius);
    width: auto;
    flex-direction: row;
  }

  .auth-tab-indicator {
    width: 50%;
    height: var(--tab-height);
    border-radius: var(--tab-radius) var(--tab-radius) 0 0;
    transform: translateX(calc(var(--tab-index) * 100%));
  }

  .auth-tab-indicator::before,
  .auth-tab-indicator::after {
    top: auto;
    bottom: 0;
  }

  .auth-tab-indicator::before {
    right: 100%;
    background: radial-gradient(
      circle at 0 0,
      transparent calc(var(--tab-radius) - 0.5px),
      var(--surface) var(--tab-radius)
    );
  }

  .auth-tab-indicator::after {
    right: auto;
    left: 100%;
    background: radial-gradient(
      circle at 100% 0,
      transparent calc(var(--tab-radius) - 0.5px),
      var(--surface) var(--tab-radius)
    );
  }

  .auth-tab {
    flex: 1 1 50%;
    justify-content: center;
    height: var(--tab-height);
    padding-left: 0;
    font-size: 1rem;
    border-radius: var(--tab-radius) var(--tab-radius) 0 0;
  }

  .auth-tab:hover:not(.is-active) {
    transform: translateY(-2px);
  }

  .auth-content {
    align-items: flex-start;
    padding: 28px 20px 40px;
  }

  .auth-column {
    max-width: 22rem;
  }

  .auth-brand {
    gap: 12px;
    margin-bottom: 28px;
  }

  .logo-mark {
    width: 76px;
    height: 80px;
  }

  .wordmark {
    font-size: 2rem;
    line-height: 38px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-tab-indicator,
  .auth-tab,
  .auth-stage,
  .auth-swap-enter-active,
  .auth-swap-leave-active {
    transition-duration: 1ms;
  }
}
</style>
