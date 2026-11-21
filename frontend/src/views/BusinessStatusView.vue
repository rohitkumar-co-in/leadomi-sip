<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import api from '@/api/client'
import { list, dateTime, auditLabel } from '@/utils/display'

const status = ref<Record<string, string | boolean>>({})
const audit = ref<{ id: number; action: string; subject: string; outcome: string; created_at: string }[]>([])
const error = ref(''), notice = ref(''), busy = ref(false), loaded = ref(false)
const backups = ref<{ filename: string; created_at: string; size: number }[]>([])
interface Phone { username: string; number: string; state: string; online: boolean }
const phones = ref<Phone[]>([])
const automation = ref({ demo_mode: false, ai_enabled: false, key_set: false, missed_call_url_set: false, sms_url_set: false })

async function load() {
  error.value = ''
  try {
    const [s, a, b, p, au] = await Promise.all([
      api.get('/business/status'), api.get('/business/audit'), api.get('/system/backups'), api.get('/business/accounts'),
      api.get('/business/automation').catch(() => ({ data: automation.value })),
    ])
    status.value = s.data; audit.value = list(a.data.data); backups.value = list(b.data); phones.value = list(p.data.data)
    automation.value = au.data; loaded.value = true
  } catch { error.value = 'Could not load service status' }
}
async function backup() { busy.value = true; error.value = ''; try { await api.post('/system/backups'); notice.value = 'Backup created'; await load() } catch { error.value = 'Backup could not complete' } finally { busy.value = false } }
async function verify(filename: string) { busy.value = true; try { await api.post('/system/backups/verify', { filename }); notice.value = 'Backup verified: database is intact' } catch { error.value = 'Backup verification failed' } finally { busy.value = false } }
interface Check { id: string; label: string; status: 'ok' | 'fail' | 'warn' | 'skip'; detail?: string; fix?: string }
const checks = ref<Check[]>([]), checking = ref(false), checkError = ref(''), checkedAt = ref('')
async function runChecks() {
  checking.value = true; checkError.value = ''
  try { const res = await api.get('/business/setup-check', { timeout: 60000 }); checks.value = res.data.checks || []; checkedAt.value = new Date().toLocaleTimeString('en-GB') }
  catch { checkError.value = 'Setup check could not run. Try again.' } finally { checking.value = false }
}
const icon = (s: string) => ({ ok: '✓', fail: '✗', warn: '!', skip: '–' } as Record<string, string>)[s] || '?'
const iconClass = (s: string) => ({ ok: 'bg-green-600 text-white', fail: 'bg-red-600 text-white', warn: 'bg-amber-500 text-black', skip: 'bg-gray-600 text-white' } as Record<string, string>)[s] || ''
const problems = () => checks.value.filter(c => c.status === 'fail').length
const connected = () => automation.value.key_set && automation.value.missed_call_url_set && automation.value.sms_url_set
onMounted(() => { load(); runChecks() })
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div><h1 class="text-2xl font-semibold">Dashboard</h1><p class="text-sm text-gray-500 mt-2">Status of your business phone line at a glance.</p></div>
      <button @click="load" class="border rounded px-4 py-2">Refresh</button>
    </div>
    <p v-if="error" role="alert" class="text-red-500">{{ error }}</p>
    <p v-if="notice" role="status" class="text-green-500">{{ notice }}</p>

    <div v-if="loaded && automation.demo_mode" role="alert" class="rounded-lg border border-amber-500 bg-amber-500/10 p-4 flex flex-wrap items-center justify-between gap-3">
      <div><p class="font-semibold text-amber-400">Demo mode is ON — your phone will not ring</p><p class="text-sm text-amber-200/80">Every call is rejected and the caller gets the text-back. Turn it off for real customers.</p></div>
      <RouterLink to="/settings" class="border border-amber-500 text-amber-300 rounded px-3 py-2 text-sm">Turn off in Settings</RouterLink>
    </div>

    <div class="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="bg-white dark:bg-gray-800 rounded-lg p-5">
        <p class="text-gray-500 text-sm">Phone</p>
        <p :class="['font-semibold mt-2', !loaded ? '' : phones[0]?.online ? 'text-green-400' : 'text-amber-400']">{{ !loaded ? 'Checking…' : phones[0]?.online ? '● Connected' : '○ Not connected' }}</p>
        <RouterLink v-if="loaded && !phones[0]?.online" to="/phone-setup" class="text-primary text-xs">Set up Linphone →</RouterLink>
      </div>
      <div class="bg-white dark:bg-gray-800 rounded-lg p-5">
        <p class="text-gray-500 text-sm">Phone server</p>
        <p :class="['font-semibold mt-2', !loaded ? '' : status.pbx_healthy ? 'text-green-400' : 'text-red-400']">{{ !loaded ? 'Checking…' : status.pbx_healthy ? '● Running' : '● Needs attention' }}</p>
        <p class="text-xs text-gray-500">{{ status.twilio_configured ? 'Twilio connected' : 'Twilio not configured' }}</p>
      </div>
      <div class="bg-white dark:bg-gray-800 rounded-lg p-5">
        <p class="text-gray-500 text-sm">AI replies</p>
        <p :class="['font-semibold mt-2', automation.ai_enabled ? 'text-green-400' : 'text-gray-400']">{{ !loaded ? 'Checking…' : automation.ai_enabled ? '● On' : '○ Off' }}</p>
        <p class="text-xs text-gray-500">{{ connected() ? 'Only for people who called' : 'n8n not connected' }}</p>
      </div>
      <div class="bg-white dark:bg-gray-800 rounded-lg p-5">
        <p class="text-gray-500 text-sm">Incoming calls</p>
        <p :class="['font-semibold mt-2', automation.demo_mode ? 'text-amber-400' : 'text-green-400']">{{ !loaded ? 'Checking…' : automation.demo_mode ? 'Demo mode' : '● Ring phone' }}</p>
        <p class="text-xs text-gray-500">{{ automation.demo_mode ? 'Rejected + instant text-back' : 'Text-back only if missed' }}</p>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-lg p-5 space-y-3">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 class="font-semibold">Setup check</h2>
          <p class="text-sm mt-1" :class="checking && !checks.length ? 'text-gray-500' : problems() ? 'text-red-400' : 'text-green-400'">
            {{ checking && !checks.length ? 'Checking Twilio and the server…' : problems() ? problems() + ' item(s) need fixing' : 'Everything is set up' }}
            <span v-if="checkedAt" class="text-gray-500"> · checked {{ checkedAt }}</span>
          </p>
        </div>
        <button @click="runChecks" :disabled="checking" class="border rounded px-3 py-2 text-sm">{{ checking ? 'Checking…' : 'Re-check' }}</button>
      </div>
      <p v-if="checkError" role="alert" class="text-sm text-red-400">{{ checkError }}</p>
      <ul class="divide-y divide-gray-200 dark:divide-gray-700">
        <li v-for="c in checks" :key="c.id" class="py-2 flex gap-3">
          <span :class="['shrink-0 h-6 w-6 rounded-full flex items-center justify-center text-xs font-bold', iconClass(c.status)]" :aria-label="c.status">{{ icon(c.status) }}</span>
          <div class="min-w-0">
            <p class="text-sm font-medium">{{ c.label }}</p>
            <p v-if="c.detail" class="text-xs text-gray-500 break-words">{{ c.detail }}</p>
            <p v-if="c.fix && c.status !== 'ok'" class="text-xs text-amber-400 break-words">Fix: {{ c.fix }}</p>
          </div>
        </li>
      </ul>
      <p class="text-xs text-gray-500">Read-only: this only looks at Twilio and the server, it never changes anything.</p>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-lg p-5 flex flex-wrap items-center justify-between gap-4">
      <div>
        <h2 class="font-semibold">Business number</h2>
        <p v-for="p in phones" :key="p.username" class="mt-1"><span class="font-mono text-lg">{{ p.number || 'Not assigned yet' }}</span> <span class="text-gray-500 text-sm">· phone user {{ p.username }}</span></p>
        <p v-if="loaded && !phones.length" class="text-sm text-amber-400 mt-1">The phone user is being set up from the server settings. Refresh in a minute.</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <RouterLink to="/phone-setup" class="border rounded px-3 py-2 text-sm">Phone setup</RouterLink>
        <RouterLink to="/dids" class="border rounded px-3 py-2 text-sm">Change number</RouterLink>
        <RouterLink to="/simulator" class="border rounded px-3 py-2 text-sm">Test in Simulator</RouterLink>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-lg p-5 space-y-3">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div><h2 class="font-semibold">Backups</h2><p class="text-sm mt-1">Last daily backup: {{ dateTime(typeof status.last_backup === 'string' ? status.last_backup : undefined) }}</p></div>
        <button @click="backup" :disabled="busy" class="border rounded px-3 py-2">Create backup now</button>
      </div>
      <p class="text-xs text-gray-500">Daily server backups include the database and phone credentials. To restore a whole server you also need the Coolify environment variables.</p>
      <div v-for="b in backups.slice(0, 10)" :key="b.filename" class="flex flex-wrap justify-between gap-2 border-t pt-2 text-sm">
        <span>{{ b.filename }} · {{ Math.round(b.size / 1024) }} KB</span><button @click="verify(b.filename)" :disabled="busy" class="text-primary">Verify</button>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-lg p-5">
      <h2 class="font-semibold mb-3">Recent changes</h2>
      <div v-for="a in audit.slice(0, 15)" :key="a.id" class="border-t border-gray-200 dark:border-gray-700 py-2 text-sm flex flex-wrap justify-between gap-2">
        <span>{{ auditLabel(a.action) }} <span class="text-gray-500">· {{ a.subject }}</span></span>
        <span class="text-gray-500">{{ a.outcome === 'ready' ? 'Done' : a.outcome.replace(/_/g, ' ') }} · {{ dateTime(a.created_at) }}</span>
      </div>
      <p v-if="!audit.length" class="text-sm text-gray-500">No changes recorded yet.</p>
    </div>
  </div>
</template>
