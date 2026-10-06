import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import HomeFeed from '../views/HomeFeed.vue'
import Login from '../views/Login.vue'
import Register from '../views/Register.vue'
import Profile from '../views/Profile.vue'
import Groups from '../views/Groups.vue'
import GroupDetail from '../views/GroupDetail.vue'
import Chat from '../views/Chat.vue'
import Notifications from '../views/Notifications.vue'
import PostComposer from '../views/PostComposer.vue'
import PostDetail from '../views/PostDetail.vue'

const routes = [
  { path: '/', name: 'feed', component: HomeFeed },
  { path: '/posts/new', name: 'post-new', component: PostComposer },
  { path: '/posts/:id', name: 'post-detail', component: PostDetail },
  { path: '/login', name: 'login', component: Login, meta: { public: true } },
  { path: '/register', name: 'register', component: Register, meta: { public: true } },
  { path: '/profile/:id', name: 'profile', component: Profile },
  { path: '/groups', name: 'groups', component: Groups },
  { path: '/groups/:id', name: 'group-detail', component: GroupDetail },
  { path: '/chat', name: 'chat', component: Chat },
  { path: '/chat/:userId', name: 'chat-with', component: Chat },
  { path: '/notifications', name: 'notifications', component: Notifications },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  // A page whose content loads after this runs (the feed) has to reapply the saved position itself.
  scrollBehavior(to, from, savedPosition) {
    return savedPosition || { top: 0 }
  },
})

// The guard waits for checkSession: the first navigation starts before /api/me has answered, so deciding then would send a valid session to /login.
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.checkSession()

  if (to.meta.public) {
    return auth.isLoggedIn ? { name: 'feed' } : true
  }

  return auth.isLoggedIn ? true : { name: 'login' }
})

export default router
