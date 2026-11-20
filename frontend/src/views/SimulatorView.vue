<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from '@/api/client'
import { PhoneMissed, Send, RefreshCw } from 'lucide-vue-next'

interface Msg { id: number; direction: string; body: string; status: string; created_at: string }
const customer = ref('+447700900123')
const didId = ref(0)
const messages = ref<Msg[]>([])
const draft = ref('')
const busy = ref(false)
const notice = ref('')
const error = ref('')
const automation = ref({ demo_mode: false, ai_enabled: false, key_set: false, missed_call_url_set: false, sms_url_set: false })
const valid = computed(() => /^\+447700900\d{3}$/.test(customer.value))
const sorted = computed(() => [...messages.value].sort((a, b) => a.created_at.localeCompare(b.created_at) || a.id - b.id))

function errorText(e: unknown, fallback: string) {
  return (e as { response?: { data?: { error?: { message?: string } } } }).response?.data?.error?.message || fallback
}
function newCustomer() {
  customer.value = '+447700900' + String(Math.floor(Math.random() * 1000)).padStart(3, '0')
  messages.value = []
  notice.value = ''
}
async function loadThread() {
  if (!valid.value || !didId.value) return
  try {
    const res = await api.get(`/messages/conversation/${encodeURIComponent(customer.value)}`, { params: { did_id: didId.value, limit: 50 } })
    messages.value = res.data.data || []
  } catch { /* keep the last thread on transient errors */ }
}
async function simulate(action: 'missed_call' | 'sms') {
  if (!valid.value || busy.value) return
  if (action === 'sms' && !draft.value.trim()) return
  busy.value = true; error.value = ''; notice.value = ''
  try {
    const res = await api.post('/business/simulator', { action, from: customer.value, body: draft.value })
    didId.value = res.data.did_id
    if (action === 'sms') draft.value = ''
    notice.value = action === 'missed_call'
      ? (res.data.automation_connected ? 'Missed call simulated. The text-back should appear in a few seconds.' : 'Missed call recorded, but n8n is not connected, so no text-back will be sent.')
      : (res.data.ai_enabled ? 'Message sent as the customer. If this number called first, the AI replies in a few seconds.' : 'Message sent as the customer. AI replies are off in Settings, so only you (phone / Messages) can reply.')
    await loadThread()
  } catch (e) {
    error.value = errorText(e, 'Simulation failed')
  } finally {
    busy.value = false
  }
}
function time(value: string) {
  const d = new Date(value)
  return isNaN(d.getTime()) ? '' : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
let timer: ReturnType<typeof setInterval>
onMounted(async () => {
  try { automation.value = (await api.get('/business/automation')).data } catch { /* optional */ }
  try {
    const dids = (await api.get('/dids')).data.data || []
    if (dids.length) didId.value = dids[0].id
  } catch { /* resolved on first simulation */ }
  loadThread()
  timer = setInterval(loadThread, 2000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-semibold">Simulator</h1>
      <p class="text-sm text-gray-500 mt-2">Act as a customer from a fictional UK number (Ofcom's reserved +447700900xxx range). Calls and texts run through the real server, n8n and AI, and appear in Messages and on your phone, but replies to these numbers are never sent by Twilio, so nothing is charged or delivered to a real person.</p>
    </div>
    <div class="flex flex-wrap gap-2 text-xs">
      <span :class="['rounded px-2 py-1 border', automation.key_set && automation.missed_call_url_set && automation.sms_url_set ? 'border-green-600 text-green-500' : 'border-amber-600 text-amber-500']">n8n {{ automation.key_set && automation.missed_call_url_set && automation.sms_url_set ? 'connected' : 'not connected' }}</span>
      <span :class="['rounded px-2 py-1 border', automation.ai_enabled ? 'border-green-600 text-green-500' : 'border-gray-500 text-gray-400']">AI replies {{ automation.ai_enabled ? 'on' : 'off' }}</span>
    </div>
    <div class="bg-white dark:bg-gray-800 rounded-lg p-5 space-y-4">
      <div class="flex flex-wrap items-end gap-3">
        <label class="block text-sm">Customer number
          <input v-model="customer" @change="messages = []; loadThread()" class="block border rounded p-2 mt-1 dark:bg-gray-700" aria-label="Simulated customer number" />
        </label>
        <button @click="newCustomer" class="border rounded px-3 py-2 text-sm">New customer</button>
        <button @click="simulate('missed_call')" :disabled="busy || !valid" class="bg-primary text-white rounded px-4 py-2 text-sm inline-flex items-center gap-2"><PhoneMissed class="h-4 w-4" />Simulate missed call</button>
        <button @click="loadThread" class="border rounded px-3 py-2 text-sm inline-flex items-center gap-2" aria-label="Refresh conversation"><RefreshCw class="h-4 w-4" /></button>
      </div>
      <p v-if="!valid" class="text-sm text-amber-500">Use a number from +447700900000 to +447700900999.</p>
      <p v-if="notice" role="status" class="text-sm text-green-500">{{ notice }}</p>
      <p v-if="error" role="alert" class="text-sm text-red-500">{{ error }}</p>
    </div>
    <div class="bg-white dark:bg-gray-800 rounded-lg flex flex-col" style="min-height: 22rem">
      <div class="p-4 border-b border-gray-200 dark:border-gray-700 text-sm font-semibold">Conversation with {{ customer }}</div>
      <div class="flex-1 p-4 space-y-3 overflow-y-auto">
        <div v-for="m in sorted" :key="m.id" :class="['flex', m.direction === 'inbound' ? 'justify-end' : 'justify-start']">
          <div :class="['message-bubble rounded-lg px-3 py-2 text-sm', m.direction === 'inbound' ? 'bg-primary text-white' : 'bg-gray-100 dark:bg-gray-700']">
            <p class="whitespace-pre-wrap">{{ m.body }}</p>
            <p class="text-xs opacity-70 mt-1">{{ m.direction === 'inbound' ? 'Customer' : 'Business' }} · {{ time(m.created_at) }} · {{ m.status }}</p>
          </div>
        </div>
        <p v-if="!sorted.length" class="text-sm text-gray-500">No messages yet. Simulate a missed call to start the text-back demo, or send a message as the customer.</p>
      </div>
      <form @submit.prevent="simulate('sms')" class="message-composer single-number p-4 border-t border-gray-200 dark:border-gray-700">
        <input v-model="draft" :disabled="!valid" placeholder="Reply as the customer, e.g. My car needs an MOT" aria-label="Message as the customer" class="px-3 py-2 border rounded-md dark:bg-gray-700" />
        <button :disabled="busy || !valid || !draft.trim()" class="px-4 py-2 bg-primary text-white rounded-md inline-flex items-center gap-2" aria-label="Send as customer"><Send class="h-4 w-4" /></button>
      </form>
    </div>
    <p class="text-sm text-gray-500">Customer messages are on the right. Business replies (text-back, AI, your phone or Messages) are on the left. AI only answers numbers that "called" first, exactly as with real customers.</p>
  </div>
</template>
