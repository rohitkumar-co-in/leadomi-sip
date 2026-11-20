<script setup lang="ts">
import { RouterView, RouterLink, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  Phone,
  MessageSquare,
  Voicemail,
  Settings,
  Users,
  LayoutDashboard,
  LogOut,
  Menu,
  X
} from 'lucide-vue-next'
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const sidebarOpen = ref(false)
const drawer=ref<HTMLElement|null>(null),menuButton=ref<HTMLElement|null>(null)
let previousOverflow=''
watch(sidebarOpen,(open)=>{if(open){previousOverflow=document.body.style.overflow;document.body.style.overflow='hidden';drawer.value?.querySelector('button')?.focus()}else{document.body.style.overflow=previousOverflow;menuButton.value?.focus()}},{flush:'post'})
function resize(){if(window.innerWidth>=1024)sidebarOpen.value=false}
function drawerKey(event:KeyboardEvent){
 if(event.key==='Escape'){sidebarOpen.value=false;return}
 if(event.key!=='Tab')return
 const controls=Array.from(drawer.value?.querySelectorAll<HTMLElement>('a[href],button:not(:disabled)')||[])
 const first=controls[0],last=controls[controls.length-1]
 if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus()}
 else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus()}
}
onMounted(()=>window.addEventListener('resize',resize))
onBeforeUnmount(()=>{window.removeEventListener('resize',resize);if(sidebarOpen.value)document.body.style.overflow=previousOverflow})

const navigation = [
  { name: 'Dashboard', href: '/', icon: LayoutDashboard },
  { name: 'Phone Numbers', href: '/dids', icon: Phone },
  { name: 'Activity Log', href: '/activity', icon: LayoutDashboard },
  { name: 'Messages', href: '/messages', icon: MessageSquare },
  { name: 'Voicemails', href: '/voicemails', icon: Voicemail },
  { name: 'Settings', href: '/settings', icon: Settings }
]

const adminNavigation = [
  { name: 'Administrators', href: '/users', icon: Users }
]

function isActive(href: string) {
  if (href === '/') {
    return route.path === '/'
  }
  return route.path.startsWith(href)
}

async function handleLogout() {
  await authStore.logout()
  await router.replace('/login')
}
</script>

<template>
  <div class="min-h-screen bg-background">
    <!-- Mobile sidebar -->
    <div v-if="sidebarOpen" class="fixed inset-0 z-40 lg:hidden">
      <div class="fixed inset-0 bg-black/75" @click="sidebarOpen = false" />
      <div ref="drawer" role="dialog" aria-modal="true" aria-label="Navigation" @keydown="drawerKey" class="mobile-drawer fixed inset-y-0 left-0 flex w-64 flex-col bg-white dark:bg-gray-800">
        <div class="flex h-16 shrink-0 items-center justify-between px-4">
          <RouterLink to="/" class="brand-wordmark" aria-label="Leadomi SIP dashboard" @click="sidebarOpen = false">Leadomi SIP</RouterLink>
          <button @click="sidebarOpen = false" aria-label="Close navigation" class="p-3 text-gray-500">
            <X class="h-6 w-6" />
          </button>
        </div>
        <nav class="min-h-0 flex-1 overflow-y-auto space-y-1 px-2 py-4">
          <RouterLink
            v-for="item in navigation"
            :key="item.name"
            :to="item.href"
            :class="[
              isActive(item.href)
                ? 'bg-primary/10 text-primary'
                : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700',
              'group flex items-center min-h-11 px-3 py-2 text-sm font-medium rounded-md'
            ]"
            @click="sidebarOpen = false"
          >
            <component
              :is="item.icon"
              :class="[
                isActive(item.href) ? 'text-primary' : 'text-gray-400 group-hover:text-gray-500',
                'mr-3 h-5 w-5 flex-shrink-0'
              ]"
            />
            {{ item.name }}
          </RouterLink>

          <template v-if="authStore.isAdmin">
            <div class="pt-4 pb-2">
              <p class="px-3 text-xs font-semibold text-gray-400 uppercase">Admin</p>
            </div>
            <RouterLink
              v-for="item in adminNavigation"
              :key="item.name"
              :to="item.href"
              :class="[
                isActive(item.href)
                  ? 'bg-primary/10 text-primary'
                  : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700',
                'group flex items-center min-h-11 px-3 py-2 text-sm font-medium rounded-md'
              ]"
              @click="sidebarOpen = false"
            >
              <component
                :is="item.icon"
                :class="[
                  isActive(item.href) ? 'text-primary' : 'text-gray-400 group-hover:text-gray-500',
                  'mr-3 h-5 w-5 flex-shrink-0'
                ]"
              />
              {{ item.name }}
            </RouterLink>
          </template>
        </nav>
        <div class="shrink-0 border-t p-4 space-y-2"><p class="text-sm break-all">{{authStore.user?.email}}</p><button @click="sidebarOpen=false;handleLogout()" class="flex items-center gap-2 min-h-11 w-full text-gray-500"><LogOut class="h-5 w-5"/>Sign out</button></div>
      </div>
    </div>

    <!-- Desktop sidebar -->
    <div class="hidden lg:fixed lg:inset-y-0 lg:flex lg:w-64 lg:flex-col">
      <div class="flex min-h-0 flex-1 flex-col border-r border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
        <div class="flex h-16 shrink-0 items-center px-4 border-b border-gray-200 dark:border-gray-700">
          <RouterLink to="/" class="brand-wordmark" aria-label="Leadomi SIP dashboard">Leadomi SIP</RouterLink>
        </div>
        <nav class="min-h-0 flex-1 overflow-y-auto space-y-1 px-2 py-4">
          <RouterLink
            v-for="item in navigation"
            :key="item.name"
            :to="item.href"
            :class="[
              isActive(item.href)
                ? 'bg-primary/10 text-primary'
                : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700',
              'group flex items-center min-h-11 px-3 py-2 text-sm font-medium rounded-md'
            ]"
          >
            <component
              :is="item.icon"
              :class="[
                isActive(item.href) ? 'text-primary' : 'text-gray-400 group-hover:text-gray-500',
                'mr-3 h-5 w-5 flex-shrink-0'
              ]"
            />
            {{ item.name }}
          </RouterLink>

          <template v-if="authStore.isAdmin">
            <div class="pt-4 pb-2">
              <p class="px-3 text-xs font-semibold text-gray-400 uppercase">Admin</p>
            </div>
            <RouterLink
              v-for="item in adminNavigation"
              :key="item.name"
              :to="item.href"
              :class="[
                isActive(item.href)
                  ? 'bg-primary/10 text-primary'
                  : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700',
                'group flex items-center min-h-11 px-3 py-2 text-sm font-medium rounded-md'
              ]"
            >
              <component
                :is="item.icon"
                :class="[
                  isActive(item.href) ? 'text-primary' : 'text-gray-400 group-hover:text-gray-500',
                  'mr-3 h-5 w-5 flex-shrink-0'
                ]"
              />
              {{ item.name }}
            </RouterLink>
          </template>
        </nav>

        <div class="border-t border-gray-200 dark:border-gray-700 p-4">
          <div class="flex items-center">
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300 truncate">
                {{ authStore.user?.email }}
              </p>
              <p class="text-xs text-gray-500 truncate">
                {{ authStore.user?.role }}
              </p>
            </div>
            <button
              @click="handleLogout"
              class="ml-3 p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
              title="Sign out"
            >
              <LogOut class="h-5 w-5" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Main content -->
    <div class="lg:pl-64">
      <!-- Top bar -->
      <div class="sticky top-0 z-10 flex h-16 flex-shrink-0 bg-white dark:bg-gray-800 shadow lg:hidden">
        <button
          ref="menuButton" @click="sidebarOpen = true" aria-label="Open navigation" :aria-expanded="sidebarOpen"
          class="px-4 text-gray-500 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-primary"
        >
          <Menu class="h-6 w-6" />
        </button>
        <div class="flex flex-1 items-center justify-center">
          <RouterLink to="/" class="brand-wordmark" aria-label="Leadomi SIP dashboard">Leadomi SIP</RouterLink>
        </div>
        <div class="w-14" /> <!-- Spacer for symmetry -->
      </div>

      <main class="py-6">
        <div class="console-content mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <RouterView class="page-enter" />
        </div>
      </main>
    </div>
  </div>
</template>
