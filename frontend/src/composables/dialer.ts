import { ref, shallowRef, computed } from 'vue'
import api from '@/api/client'
import type { Device as TwilioDevice, Call as TwilioCall } from '@twilio/voice-sdk'

// One browser phone for the whole console: incoming calls ring on any page.
type State = 'off' | 'loading' | 'ready' | 'unavailable' | 'error'
const state = ref<State>('off')
const message = ref('')
const number = ref('')
const call = shallowRef<TwilioCall | null>(null)
const incoming = shallowRef<TwilioCall | null>(null)
const callState = ref<'idle' | 'connecting' | 'ringing' | 'open'>('idle')
const muted = ref(false)
const startedAt = ref(0)
const peer = ref('')
let device: TwilioDevice | null = null
let refresh: ReturnType<typeof setTimeout> | undefined

async function fetchToken(): Promise<string | null> {
  const res = await api.get('/business/dialer/token')
  if (!res.data.configured) return null
  number.value = res.data.number || ''
  clearTimeout(refresh)
  refresh = setTimeout(async () => { try { const t = await fetchToken(); if (t && device) device.updateToken(t) } catch { /* retried on next action */ } }, Math.max(60, (res.data.ttl || 3600) - 300) * 1000)
  return res.data.token
}

function track(c: TwilioCall, who: string) {
  call.value = c; peer.value = who; muted.value = false
  callState.value = 'connecting'
  c.on('ringing', () => { callState.value = 'ringing' })
  c.on('accept', () => { callState.value = 'open'; startedAt.value = Date.now() })
  const end = () => { if (call.value === c) { call.value = null; callState.value = 'idle'; startedAt.value = 0 } }
  c.on('disconnect', end); c.on('cancel', end); c.on('reject', end)
  c.on('error', (e: { message?: string }) => { message.value = e?.message || 'Call failed'; end() })
}

async function start() {
  if (device || state.value === 'loading') return
  state.value = 'loading'; message.value = ''
  try {
    const token = await fetchToken()
    if (!token) { state.value = 'unavailable'; return }
    const { Device } = await import('@twilio/voice-sdk')
    device = new Device(token, { codecPreferences: ['opus', 'pcmu'] as never, closeProtection: true })
    device.on('registered', () => { state.value = 'ready' })
    device.on('error', (e: { message?: string }) => { message.value = e?.message || 'Dialer error' })
    device.on('tokenWillExpire', async () => { try { const t = await fetchToken(); if (t) device?.updateToken(t) } catch { /* ignore */ } })
    device.on('incoming', (c: TwilioCall) => {
      if (call.value) { c.reject(); return }
      incoming.value = c
      const clear = () => { if (incoming.value === c) incoming.value = null }
      c.on('cancel', clear); c.on('disconnect', clear); c.on('reject', clear)
    })
    await device.register()
  } catch {
    state.value = 'error'; message.value = 'The browser dialer could not start. Allow microphone access and reload.'
    device?.destroy(); device = null
  }
}

async function dial(to: string) {
  message.value = ''
  if (!device) await start()
  if (!device || call.value) return
  try { track(await device.connect({ params: { To: to } }), to) }
  catch { message.value = 'Could not place the call. Allow microphone access and try again.' }
}
function accept() { const c = incoming.value; if (!c) return; incoming.value = null; track(c, c.parameters?.From || 'Caller'); c.accept(); callState.value = 'open'; startedAt.value = Date.now() }
function decline() { incoming.value?.reject(); incoming.value = null }
function hangup() { call.value?.disconnect() }
function toggleMute() { if (call.value) { muted.value = !muted.value; call.value.mute(muted.value) } }
function digits(d: string) { call.value?.sendDigits(d) }

export function useDialer() {
  return { state, message, number, call, incoming, callState, muted, startedAt, peer, inCall: computed(() => !!call.value), start, dial, accept, decline, hangup, toggleMute, digits }
}
