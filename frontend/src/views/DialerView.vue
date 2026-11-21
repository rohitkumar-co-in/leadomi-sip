<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { Phone, PhoneOff, Mic, MicOff, Delete, Grid3x3, PhoneIncoming, PhoneOutgoing, PhoneMissed, Settings2, MessageSquare } from 'lucide-vue-next'
import api from '@/api/client'
import { list, dateTime } from '@/utils/display'
import { useDialer } from '@/composables/dialer'

const d = useDialer()
const entry = ref('+44')
const now = ref(Date.now())
const showKeys = ref(false)
const showAudio = ref(false)
interface Recent { id: number; direction: string; from_number: string; to_number: string; status: string; occurred_at: string; duration: number }
const recent = ref<Recent[]>([])
let tick: ReturnType<typeof setInterval>

async function loadRecent() {
  try { const res = await api.get('/business/activity', { params: { kind: 'call', offset: 0 } }); recent.value = list<Recent>(res.data.data).slice(0, 15) } catch { /* keep list */ }
}
function onKey(e: KeyboardEvent) {
  const t = e.target as HTMLElement
  if (t && (t.tagName === 'INPUT' || t.tagName === 'SELECT' || t.tagName === 'TEXTAREA')) return
  if (/^[0-9*#+]$/.test(e.key)) { press(e.key); e.preventDefault() }
  else if (e.key === 'Backspace' && !d.inCall.value) back()
  else if (e.key === 'Enter' && !d.inCall.value) call()
  else if (e.key === 'Escape' && d.inCall.value) d.hangup()
}
const route = useRoute()
// Deep link from other tools: /dialer?to=+447700900123 prefills the number.
const linked = typeof route.query.to === 'string' ? clean(route.query.to) : ''
if (/^\+[1-9]\d{7,14}$/.test(linked)) entry.value = linked
onMounted(() => { d.start(); loadRecent(); tick = setInterval(() => { now.value = Date.now() }, 1000); window.addEventListener('keydown', onKey) })
onUnmounted(() => { clearInterval(tick); window.removeEventListener('keydown', onKey) })
watch(() => d.inCall.value, (on) => { if (!on) { showKeys.value = false; setTimeout(loadRecent, 2500) } })

const keys: [string, string][] = [['1', ''], ['2', 'ABC'], ['3', 'DEF'], ['4', 'GHI'], ['5', 'JKL'], ['6', 'MNO'], ['7', 'PQRS'], ['8', 'TUV'], ['9', 'WXYZ'], ['*', ''], ['0', '+'], ['#', '']]
function press(k: string) {
  if (d.inCall.value) { if (k !== '+') d.digits(k); return }
  if (k === '+' && entry.value.includes('+')) return
  entry.value += k
}
function back() { entry.value = entry.value.slice(0, -1) }
function clean(v: string) { return v.replace(/[^\d+]/g, '').replace(/^00/, '+') }
const cleaned = computed(() => clean(entry.value))
const valid = computed(() => /^\+[1-9]\d{7,14}$/.test(cleaned.value))
function call(to?: string) {
  const n = to || cleaned.value
  if (!/^\+[1-9]\d{7,14}$/.test(n) || d.inCall.value) return
  entry.value = n; d.dial(n)
}
function onPaste(e: ClipboardEvent) { const t = e.clipboardData?.getData('text'); if (t) { e.preventDefault(); entry.value = clean(t) } }
const elapsed = computed(() => {
  if (!d.startedAt.value) return ''
  const s = Math.max(0, Math.floor((now.value - d.startedAt.value) / 1000))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
})
const status = computed(() => ({ idle: '', connecting: 'Calling…', ringing: 'Ringing…', open: elapsed.value } as Record<string, string>)[d.callState.value])
const other = (c: Recent) => (c.direction === 'inbound' ? c.from_number : c.to_number)
const missed = (c: Recent) => c.direction === 'inbound' && ['no-answer', 'busy', 'canceled', 'missed', 'failed'].includes(c.status)
const dur = (s: number) => (s ? `${Math.floor(s / 60)}m ${s % 60}s` : '')
const ready = computed(() => d.state.value === 'ready')
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold">Dialer</h1>
        <p class="text-sm text-gray-500 mt-2">Call from this browser. Callers see <span class="font-mono">{{ d.number.value || 'your business number' }}</span>.</p>
      </div>
      <span v-if="d.state.value !== 'unavailable'" :class="['rounded-full border px-3 py-1 text-xs', ready ? 'border-green-600 text-green-400' : 'border-amber-500 text-amber-400']" role="status">
        {{ ready ? '● Ready · incoming calls ring here and on Linphone' : d.state.value === 'error' ? '○ Not connected' : '○ Connecting…' }}
      </span>
    </div>

    <div v-if="d.state.value === 'unavailable'" role="alert" class="rounded-lg border border-amber-500 bg-amber-500/10 p-4 text-sm max-w-xl">
      <p class="font-semibold text-amber-400">The browser dialer is not set up yet</p>
      <p class="mt-1">Add <span class="font-mono">TWILIO_API_KEY_SID</span>, <span class="font-mono">TWILIO_API_KEY_SECRET</span> and <span class="font-mono">TWILIO_TWIML_APP_SID</span> in Coolify and redeploy, then run the Setup check on the <RouterLink to="/" class="text-primary">Dashboard</RouterLink>.</p>
    </div>

    <div v-else class="grid lg:grid-cols-[minmax(0,380px)_1fr] gap-6 items-start">
      <!-- Phone -->
      <div class="bg-white dark:bg-gray-800 rounded-2xl p-5 space-y-5 shadow-sm">
        <div v-if="d.inCall.value" class="text-center pt-4 pb-2 space-y-2">
          <div class="mx-auto h-20 w-20 rounded-full bg-primary/15 text-primary flex items-center justify-center" :class="d.callState.value !== 'open' ? 'animate-pulse' : ''">
            <component :is="d.direction.value === 'incoming' ? PhoneIncoming : PhoneOutgoing" class="h-9 w-9" />
          </div>
          <p class="font-mono text-2xl break-all">{{ d.peer.value }}</p>
          <p class="text-sm" :class="d.callState.value === 'open' ? 'text-green-400 font-mono' : 'text-gray-500'" aria-live="polite">{{ status }}</p>
          <p v-if="d.quality.value" role="status" class="text-xs text-amber-400">⚠ {{ d.quality.value }}</p>
        </div>
        <div v-else class="space-y-2">
          <label for="dial-number" class="text-xs text-gray-500">Number with country code</label>
          <div class="flex items-center gap-2">
            <input id="dial-number" v-model="entry" inputmode="tel" autocomplete="tel" @keyup.enter="call()" @paste="onPaste"
              class="flex-1 min-w-0 font-mono text-2xl bg-transparent border-b-2 border-gray-300 dark:border-gray-600 focus:border-primary outline-none px-1 py-2" placeholder="+447700900123" />
            <button type="button" @click="back" class="rounded-full p-2 hover:bg-gray-100 dark:hover:bg-gray-700" aria-label="Delete last digit"><Delete class="h-5 w-5" /></button>
          </div>
          <div class="flex gap-2 text-xs">
            <button v-for="c in ['+44', '+91', '+1']" :key="c" type="button" @click="entry = c" class="border rounded-full px-3 py-1">{{ c }}</button>
          </div>
          <p v-if="entry.length > 4 && !valid" class="text-xs text-amber-400">Use the full number with country code, e.g. +447700900123.</p>
        </div>

        <div v-if="!d.inCall.value || showKeys" class="grid grid-cols-3 gap-3 px-2">
          <button v-for="[k, sub] in keys" :key="k" type="button" @click="press(k)" @contextmenu.prevent="k === '0' && press('+')"
            class="aspect-square max-h-20 rounded-full border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-white/5 hover:bg-gray-200 dark:hover:bg-white/10 active:scale-95 transition flex flex-col items-center justify-center" :aria-label="k">
            <span class="text-2xl font-medium leading-none">{{ k }}</span><span class="text-[10px] tracking-widest text-gray-500 h-3">{{ sub }}</span>
          </button>
        </div>

        <div class="flex justify-center items-center gap-5 pb-1">
          <template v-if="d.inCall.value">
            <button type="button" @click="d.toggleMute" :aria-pressed="d.muted.value" :aria-label="d.muted.value ? 'Unmute' : 'Mute'"
              :class="['rounded-full h-14 w-14 flex items-center justify-center border', d.muted.value ? 'bg-amber-500 text-black border-amber-500' : '']">
              <component :is="d.muted.value ? MicOff : Mic" class="h-6 w-6" />
            </button>
            <button type="button" @click="d.hangup" class="bg-red-600 hover:bg-red-700 text-white rounded-full h-16 w-16 flex items-center justify-center" aria-label="Hang up"><PhoneOff class="h-7 w-7" /></button>
            <button type="button" @click="showKeys = !showKeys" :aria-pressed="showKeys" aria-label="Keypad"
              :class="['rounded-full h-14 w-14 flex items-center justify-center border', showKeys ? 'bg-primary text-white border-primary' : '']"><Grid3x3 class="h-6 w-6" /></button>
          </template>
          <button v-else type="button" @click="call()" :disabled="!valid || !ready" class="bg-green-600 hover:bg-green-700 disabled:opacity-40 text-white rounded-full h-16 w-16 flex items-center justify-center" aria-label="Call"><Phone class="h-7 w-7" /></button>
        </div>
        <p v-if="d.message.value" role="alert" class="text-sm text-red-400 text-center">{{ d.message.value }}</p>

        <div class="border-t border-gray-200 dark:border-gray-700 pt-3">
          <button type="button" @click="showAudio = !showAudio" :aria-expanded="showAudio" class="text-xs text-gray-500 inline-flex items-center gap-1"><Settings2 class="h-3.5 w-3.5" />Microphone and speaker</button>
          <div v-if="showAudio" class="mt-2 space-y-2 text-sm">
            <label class="block"><span class="text-xs text-gray-500">Microphone</span>
              <select :value="d.inputId.value" @change="d.setInput(($event.target as HTMLSelectElement).value)" class="w-full border rounded px-2 py-1 bg-transparent">
                <option v-for="o in d.inputs.value" :key="o.id" :value="o.id">{{ o.label }}</option>
              </select>
            </label>
            <label v-if="d.outputs.value.length" class="block"><span class="text-xs text-gray-500">Speaker</span>
              <select :value="d.outputId.value" @change="d.setOutput(($event.target as HTMLSelectElement).value)" class="w-full border rounded px-2 py-1 bg-transparent">
                <option v-for="o in d.outputs.value" :key="o.id" :value="o.id">{{ o.label }}</option>
              </select>
            </label>
            <p class="text-xs text-gray-500">Names appear after you allow microphone access (make one call).</p>
          </div>
        </div>
      </div>

      <!-- Recent calls -->
      <div class="bg-white dark:bg-gray-800 rounded-2xl p-5">
        <div class="flex items-center justify-between mb-3">
          <h2 class="font-semibold">Recent calls</h2>
          <RouterLink to="/activity" class="text-xs text-primary">All activity →</RouterLink>
        </div>
        <ul class="divide-y divide-gray-200 dark:divide-gray-700">
          <li v-for="c in recent" :key="c.id" class="py-2 flex items-center gap-3">
            <component :is="missed(c) ? PhoneMissed : c.direction === 'inbound' ? PhoneIncoming : PhoneOutgoing" :class="['h-4 w-4 shrink-0', missed(c) ? 'text-red-400' : 'text-gray-500']" :aria-label="missed(c) ? 'Missed' : c.direction" />
            <div class="min-w-0 flex-1">
              <p class="font-mono text-sm truncate" :class="missed(c) ? 'text-red-400' : ''">{{ other(c) }}</p>
              <p class="text-xs text-gray-500">{{ dateTime(c.occurred_at) }}<span v-if="dur(c.duration)"> · {{ dur(c.duration) }}</span></p>
            </div>
            <RouterLink :to="{ path: '/messages', query: { to: other(c) } }" class="rounded-full p-2 hover:bg-gray-100 dark:hover:bg-gray-700" :aria-label="'Text ' + other(c)"><MessageSquare class="h-4 w-4" /></RouterLink>
            <button type="button" @click="call(other(c))" :disabled="d.inCall.value || !ready" class="rounded-full p-2 text-green-500 hover:bg-green-500/10 disabled:opacity-40" :aria-label="'Call ' + other(c)"><Phone class="h-4 w-4" /></button>
          </li>
        </ul>
        <p v-if="!recent.length" class="text-sm text-gray-500">No calls yet.</p>
        <p class="text-xs text-gray-500 mt-4">Tip: type digits, Enter to call, Esc to hang up. Keep a console tab open to receive calls; on a locked phone use Linphone.</p>
      </div>
    </div>
  </div>
</template>
