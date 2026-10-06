<script setup>
import { computed, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notifications'
import NavIcon from './NavIcon.vue'

defineEmits(['logout'])

const auth = useAuthStore()
const notifStore = useNotificationStore()
const route = useRoute()

watch(
  () => auth.user?.id,
  (userID) => {
    if (userID) {
      notifStore.fetchNotifications()
    }
  },
  { immediate: true },
)

// Route names rather than RouterLink's own active class, which follows a single route record: /groups/3 should still mark Groups.
const links = computed(() => [
  { label: 'Home', icon: 'home', to: '/', active: route.name === 'feed' },
  { label: 'Groups', icon: 'groups', to: '/groups', active: ['groups', 'group-detail'].includes(route.name) },
  { label: 'Messages', icon: 'messages', to: '/chat', active: ['chat', 'chat-with'].includes(route.name) },
  { label: 'Notifications', icon: 'notifications', to: '/notifications', active: route.name === 'notifications', badge: true },
  {
    label: 'Profile',
    icon: 'profile',
    to: `/profile/${auth.user?.id}`,
    active: route.name === 'profile' && String(route.params.id) === String(auth.user?.id),
  },
])

const composing = computed(() => route.name === 'post-new')

const unreadLabel = computed(() => (notifStore.unreadCount > 99 ? '99+' : String(notifStore.unreadCount)))
</script>

<template>
  <header class="topbar">
    <RouterLink to="/" class="brand" aria-label="Social Network home">
      <img class="brand-mark" src="/favicon.svg" alt="" width="32" height="32" />
      <span class="brand-name">Social Network</span>
    </RouterLink>

    <div class="topbar-actions">
      <RouterLink
        to="/posts/new"
        class="add-post-circle"
        :aria-current="composing ? 'page' : undefined"
        aria-label="Add post"
      >
        <NavIcon name="plus" />
      </RouterLink>
      <button type="button" class="icon-button" aria-label="Log out" @click="$emit('logout')">
        <NavIcon name="logout" />
      </button>
    </div>
  </header>

  <aside class="sidebar">
    <RouterLink to="/" class="brand sidebar-brand" aria-label="Social Network home">
      <img class="brand-mark" src="/favicon.svg" alt="" width="32" height="32" />
      <span class="brand-name">Social Network</span>
    </RouterLink>

    <nav class="nav" aria-label="Main">
      <ul>
        <li v-for="link in links" :key="link.label">
          <RouterLink
            :to="link.to"
            class="nav-link"
            :class="{ 'is-active': link.active }"
            :aria-current="link.active ? 'page' : undefined"
          >
            <NavIcon :name="link.icon" />
            <span class="nav-label">{{ link.label }}</span>
            <span v-if="link.badge && notifStore.unreadCount > 0" class="badge">
              {{ unreadLabel }}<span class="visually-hidden"> unread</span>
            </span>
          </RouterLink>
        </li>
      </ul>
    </nav>

    <div class="sidebar-action">
      <RouterLink
        to="/posts/new"
        class="add-post"
        :class="{ 'is-active': composing }"
        :aria-current="composing ? 'page' : undefined"
      >
        <span class="add-post-circle" aria-hidden="true">
          <NavIcon name="plus" />
        </span>
        <span class="add-post-label">Add post</span>
      </RouterLink>
    </div>

    <button type="button" class="logout" @click="$emit('logout')">
      <NavIcon name="logout" />
      <span class="logout-label">Log out</span>
    </button>
  </aside>
</template>

<style scoped>

.sidebar {
  position: fixed;
  top: 0;
  bottom: 0;
  left: 0;
  z-index: 10;
  display: flex;
  flex-direction: column;
  width: var(--sidebar-width);
  padding: var(--space-5) var(--space-4);
  background-color: var(--surface);
  border-right: 1px solid var(--border);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  color: var(--text);
  text-decoration: none;
}

.sidebar-brand {
  margin-bottom: var(--space-6);
  padding: 0 var(--space-3);
}

/* An <img>, not inline SVG: its internal mask and filter ids would clash with the top bar's copy, and max-width: none overrides global.css's img rule. */
.brand-mark {
  display: block;
  flex: none;
  width: 2rem;
  max-width: none;
  height: 2rem;
  object-fit: contain;
}

.brand-name {
  font-size: var(--text-lg);
  font-weight: 800;
  letter-spacing: -0.01em;
  white-space: nowrap;
}

.sidebar-brand .brand-name {
  font-size: var(--text-base);
}

.nav ul {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  margin: 0;
  padding: 0;
  list-style: none;
}

.nav-link {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-height: 2.75rem;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  color: var(--text);
  font-weight: 600;
  text-decoration: none;
}

.nav-link .nav-icon {
  color: var(--text-muted);
}

.nav-link:hover {
  background-color: var(--bg);
}

.nav-link.is-active {
  background-color: color-mix(in srgb, var(--primary) 10%, var(--surface));
  color: var(--primary);
  font-weight: 700;
}

.nav-link.is-active .nav-icon {
  color: var(--primary);
}

/* White on --accent is under 3:1, so the count is dark. */
.badge {
  margin-left: auto;
  min-width: 1.5rem;
  padding: 0 var(--space-2);
  border-radius: 999px;
  background-color: var(--accent);
  color: var(--text);
  font-size: var(--text-sm);
  font-weight: 700;
  line-height: 1.5rem;
  text-align: center;
}

.sidebar-action {
  margin-top: var(--space-5);
  padding-top: var(--space-5);
  border-top: 1px solid var(--border);
}

.add-post {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-sm);
  color: var(--text);
  font-weight: 700;
  text-decoration: none;
}

.add-post.is-active {
  color: var(--primary);
}

.add-post-circle {
  display: grid;
  flex: none;
  place-items: center;
  width: 2.75rem;
  height: 2.75rem;
  border-radius: 50%;
  background-color: var(--primary);
  color: var(--surface);
}

.add-post:hover .add-post-circle,
a.add-post-circle:hover {
  background-color: var(--primary-hover);
}

.logout {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-height: 2.75rem;
  margin-top: auto;
  padding: var(--space-2) var(--space-3);
  background: none;
  border: 0;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  font-weight: 600;
  text-align: left;
  cursor: pointer;
}

.logout:hover {
  background-color: var(--bg);
  color: var(--text);
}

.topbar {
  display: none;
}

/* :focus-visible, not :focus-within: a click leaves focus on the link and would hold the sidebar open after the pointer left. */
@media (min-width: 48rem) {
  .sidebar {
    width: var(--sidebar-collapsed-width);
    padding-inline: var(--space-3);
    /* clip, not hidden: hidden would make the column a scroll container. */
    overflow-x: clip;
    transition:
      width 180ms ease,
      box-shadow 180ms ease;
  }

  .sidebar:hover,
  .sidebar:has(:focus-visible) {
    width: var(--sidebar-width);
    box-shadow: 8px 0 24px color-mix(in srgb, var(--text) 8%, transparent);
  }

  /* 12 + 8 + half the 32px mark. */
  .sidebar-brand {
    padding-inline: var(--space-2);
  }

  /* 12 + 2 + half the 44px circle. */
  .add-post {
    padding-inline: 2px;
  }

  .nav-link,
  .add-post,
  .logout {
    white-space: nowrap;
  }

  .brand-name,
  .nav-label,
  .add-post-label,
  .logout-label {
    opacity: 0;
    transition: opacity 120ms ease;
  }

  .sidebar:hover :is(.brand-name, .nav-label, .add-post-label, .logout-label),
  .sidebar:has(:focus-visible) :is(.brand-name, .nav-label, .add-post-label, .logout-label) {
    opacity: 1;
  }

  .sidebar:not(:hover):not(:has(:focus-visible)) .badge {
    position: absolute;
    top: var(--space-1);
    left: calc(var(--space-3) + 0.875rem);
    min-width: 1.25rem;
    margin: 0;
    padding: 0 var(--space-1);
    line-height: 1.25rem;
  }
}

@media (min-width: 48rem) and (prefers-reduced-motion: reduce) {
  .sidebar,
  .brand-name,
  .nav-label,
  .add-post-label,
  .logout-label {
    transition: none;
  }
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
  border: 0;
}

@media (max-width: 47.99rem) {
  .topbar {
    position: fixed;
    top: 0;
    right: 0;
    left: 0;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: var(--topbar-height);
    padding: 0 var(--space-4);
    background-color: var(--surface);
    border-bottom: 1px solid var(--border);
  }

  .topbar .brand-mark {
    width: 1.75rem;
    height: 1.75rem;
  }

  .topbar-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .topbar .add-post-circle {
    width: 2.5rem;
    height: 2.5rem;
  }

  .icon-button {
    display: grid;
    place-items: center;
    width: 2.75rem;
    height: 2.75rem;
    background: none;
    border: 0;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    cursor: pointer;
  }

  .sidebar {
    top: auto;
    right: 0;
    flex-direction: row;
    width: auto;
    height: calc(var(--tabbar-height) + env(safe-area-inset-bottom));
    padding: 0 var(--space-2) env(safe-area-inset-bottom);
    border-top: 1px solid var(--border);
    border-right: 0;
  }

  .sidebar-brand,
  .sidebar-action,
  .logout {
    display: none;
  }

  .nav {
    flex: 1;
  }

  .nav ul {
    flex-direction: row;
    height: 100%;
    gap: 0;
  }

  .nav li {
    flex: 1;
  }

  .nav-label {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: -1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }

  .nav-link {
    justify-content: center;
    height: 100%;
    padding: 0;
    border-radius: 0;
  }

  .nav-link:hover,
  .nav-link.is-active {
    background: none;
  }

  .nav-link.is-active::before {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 3.5rem;
    height: 2.5rem;
    border-radius: var(--radius-sm);
    background-color: color-mix(in srgb, var(--primary) 10%, var(--surface));
    transform: translate(-50%, -50%);
    content: "";
  }

  .nav-link .nav-icon {
    position: relative;
  }

  .badge {
    position: absolute;
    top: var(--space-2);
    left: calc(50% + var(--space-1));
    min-width: 1.25rem;
    margin: 0;
    padding: 0 var(--space-1);
    line-height: 1.25rem;
  }
}
</style>
