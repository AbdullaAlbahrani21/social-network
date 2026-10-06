<script setup>
import { watch } from 'vue'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { useWebSocketStore } from './stores/websocket'
import AppSidebar from './components/AppSidebar.vue'
import AuthLayout from './components/AuthLayout.vue'
import NotificationToast from './components/NotificationToast.vue'

const auth = useAuthStore()
const wsStore = useWebSocketStore()
const router = useRouter()

// Watching the user's id rather than isLoggedIn: a different user logging in with no logout in between gets a new socket.
watch(
  () => auth.user?.id,
  (userID) => {
    if (userID) {
      wsStore.connect(userID)
    } else {
      wsStore.disconnect()
    }
  },
  { immediate: true },
)

// The navigation lives here, not in the store: the router imports the auth store, so the store importing the router back would be circular.
async function handleLogout() {
  await auth.logout()
  router.push('/login')
}

watch(
  () => wsStore.sessionEnded,
  (reason) => {
    if (!reason) return

    auth.endSession(reason)
    router.push('/login')
  },
)
</script>

<template>
  <div class="app" :class="{ 'app-signed-in': auth.isLoggedIn }">
    <AppSidebar v-if="auth.isLoggedIn" @logout="handleLogout" />

    <div class="app-main">
      <RouterView v-slot="{ Component, route }">
        <AuthLayout v-if="route.meta.public" :view="Component" />
        <component :is="Component" v-else />
      </RouterView>
    </div>

    <NotificationToast v-if="auth.isLoggedIn" />
  </div>
</template>

<style scoped>
.app {
  --sidebar-width: 15rem;
  --sidebar-collapsed-width: 4.5rem;
  --topbar-height: 3.5rem;
  --tabbar-height: 3.75rem;
}

.app-signed-in .app-main {
  margin-left: var(--sidebar-collapsed-width);
  padding-inline: var(--space-6);
}

@media (max-width: 47.99rem) {
  .app-signed-in .app-main {
    margin-left: 0;
    padding-inline: 0;
    padding-top: var(--topbar-height);
    padding-bottom: calc(var(--tabbar-height) + env(safe-area-inset-bottom));
  }
}
</style>