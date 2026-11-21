<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { RouterLink } from 'vue-router'
import { Phone, PhoneOff, Mic, MicOff, Delete } from 'lucide-vue-next'
import { useDialer } from '@/composables/dialer'

const d = useDialer()
const entry = ref('+44')
const now = ref(Date.now())
let tick: ReturnType<typeof setInterval>
onMounted(() => { d.start(); tick = setInterval(() => { now.value = Date.now() }, 1000) })
onUnmounted(() => clearInterval(tick))

const keys = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '*', '0', '#']
function press(k: string) { if (d.inCall.value) d.digits(k); else entry.value += k }
function back() { entry.value = entry.value.slice(0, -1) }
const cleaned = computed(() => entry.value.replace(/[^\d+]/g, '').replace(/^00/, '+'))
const valid = computed(() => /^\+[1-9]\d{7,14}$/.test(cleaned.value))
function call() { if (valid.value) d.dial(cleaned.value) }
const elapsed = computed(() => {
  if (!d.startedAt.value) return ''
  const s = Math.max(0, Math.floor((now.value - d.startedAt.value) / 1000))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
})
const status = computed(() => ({ idle: '', connecting: 'Connecting…', ringing: 'Ringing…', open: elapsed.value } as Record<string, string>)[d.callState.value])
</script>

<template>
  <div class="space-y-6 max-w-md">
    <div>
      <h1 class="text-2xl font-semibold">Dialer</h1>
      <p class="text-sm text-gray-500 mt-2">Call from this browser on any device. Callers see {{ d.number.value || 'your business number' }}. Incoming calls ring here while the console is open, and on Linphone.</p>
    </div>

    <div v-if="d.state.value === 'unavailable'" role="alert" class="rounded-lg border border-amber-500 bg-amber-500/10 p-4 text-sm">
      <p class="font-semibold text-amber-400">The browser dialer is not set up yet</p>
      <p class="mt-1">Add <span class="font-mono">TWILIO_API_KEY_SID</span>, <span class="font-mono">TWILIO_API_KEY_SECRET</span> and <span class="font-mono">TWILIO_TWIML_APP_SID</span> in Coolify and redeploy. See the Dialer section in the deployment guide, then run the Setup check on the <RouterLink to="/" class="text-primary">Dashboard</RouterLink>.</p>
    </div>

    <div v-else class="bg-white dark:bg-gray-800 rounded-lg p-5 space-y-4">
      <p class="text-xs" :class="d.state.value === 'ready' ? 'text-green-400' : 'text-amber-400'">
        {{ d.state.value === 'ready' ? '● Ready — incoming calls ring here too' : d.state.value === 'loading' ? 'Connecting…' : d.state.value === 'error' ? '○ Not connected' : '○ Starting…' }}
      </p>
      <div v-if="d.inCall.value" class="text-center py-2">
        <p class="font-mono text-xl">{{ d.peer.value }}</p>
        <p class="text-sm text-gray-500 mt-1" aria-live="polite">{{ status }}</p>
      </div>
      <div v-else class="flex items-center gap-2">
        <label for="dial-number" class="sr-only">Number</label>
        <input id="dial-number" v-model="entry" inputmode="tel" autocomplete="tel" @keyup.enter="call"
          class="flex-1 min-w-0 font-mono text-xl bg-transparent border rounded px-3 py-2" placeholder="+447700900123" />
        <button type="button" @click="back" class="border rounded p-2" aria-label="Delete last digit"><Delete class="h-5 w-5" /></button>
      </div>
      <div class="grid grid-cols-3 gap-2">
        <button v-for="k in keys" :key="k" type="button" @click="press(k)" class="border rounded-lg py-3 text-xl font-mono hover:bg-gray-100 dark:hover:bg-gray-700">{{ k }}</button>
      </div>
      <div class="flex justify-center gap-3">
        <template v-if="d.inCall.value">
          <button type="button" @click="d.toggleMute" class="border rounded-full h-14 w-14 flex items-center justify-center" :aria-label="d.muted.value ? 'Unmute' : 'Mute'" :aria-pressed="d.muted.value">
            <component :is="d.muted.value ? MicOff : Mic" class="h-6 w-6" />
          </button>
          <button type="button" @click="d.hangup" class="bg-red-600 text-white rounded-full h-14 w-14 flex items-center justify-center" aria-label="Hang up"><PhoneOff class="h-6 w-6" /></button>
        </template>
        <button v-else type="button" @click="call" :disabled="!valid" class="bg-green-600 disabled:opacity-40 text-white rounded-full h-14 w-14 flex items-center justify-center" aria-label="Call"><Phone class="h-6 w-6" /></button>
      </div>
      <p v-if="!d.inCall.value && entry.length > 3 && !valid" class="text-xs text-amber-400 text-center">Use the full international number, e.g. +447700900123 or +919876543210.</p>
      <p v-if="d.message.value" role="alert" class="text-sm text-red-400 text-center">{{ d.message.value }}</p>
    </div>
    <p class="text-xs text-gray-500">Allow microphone access when the browser asks. On a phone, keep this page open to receive calls; use Linphone when the screen is locked.</p>
  </div>
</template>
