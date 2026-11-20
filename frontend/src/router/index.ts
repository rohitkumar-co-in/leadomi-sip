import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { guest: true }
    },
    {
      path: '/setup',
      name: 'setup',
      component: () => import('@/views/SetupWizardView.vue'),
      meta: { guest: true }
    },
    {
      path: '/',
      component: () => import('@/views/layouts/MainLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'dashboard',
          component: () => import('@/views/BusinessStatusView.vue')
        },
        {
          // The phone user is configured in the server environment.
          path: 'devices',
          name: 'devices',
          redirect: '/'
        },
        {
          path: 'provisioning',
          name: 'provisioning',
          redirect: '/'
        },
        {
          path: 'dids',
          name: 'dids',
          component: () => import('@/views/BusinessNumbersView.vue')
        },
        {
          path: 'trunks',
          name: 'trunks',
          redirect: '/'
        },
        {
          path: 'routes',
          name: 'routes',
          redirect: '/'
        },
        {
          path: 'calls',
          name: 'calls',
          redirect: '/activity'
        },
        {
          path: 'call-control',
          name: 'call-control',
          redirect: '/'
        },
        {path:'excluded-numbers',name:'excluded-numbers',redirect:'/'},
        {path:'activity',name:'activity',component:()=>import('@/views/ActivityView.vue')},
        {path:'phone-setup',name:'phone-setup',component:()=>import('@/views/PhoneSetupView.vue')},
        {path:'simulator',name:'simulator',component:()=>import('@/views/SimulatorView.vue'),meta:{adminOnly:true}},
        {
          path: 'messages',
          name: 'messages',
          component: () => import('@/views/MessagesView.vue')
        },
        {
          path: 'voicemails',
          name: 'voicemails',
          component: () => import('@/views/VoicemailsView.vue')
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('@/views/BusinessSettingsView.vue')
        },
        {
          path: 'users',
          name: 'users',
          component: () => import('@/views/UsersView.vue'),
          meta: { adminOnly: true }
        }
      ]
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/NotFoundView.vue')
    }
  ]
})

router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // Wait for auth check to complete
  if (!authStore.initialized) {
    await authStore.checkAuth()
  }

  // Check if setup is required
  if (!authStore.setupCompleted && to.name !== 'setup') {
    return next({ name: 'setup' })
  }

  // Guest routes (login, setup) - redirect to dashboard if logged in
  if (to.meta.guest && authStore.isAuthenticated) {
    return next({ name: 'dashboard' })
  }

  // Protected routes - redirect to login if not authenticated
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return next({ name: 'login', query: { redirect: to.fullPath } })
  }

  // Admin-only routes
  if (to.meta.adminOnly && authStore.user?.role !== 'admin') {
    return next({ name: 'dashboard' })
  }

  next()
})

export default router
