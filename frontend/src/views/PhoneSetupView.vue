<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api from '@/api/client'
import { Copy, Check, Monitor, Smartphone, Tablet } from 'lucide-vue-next'

type Platform = 'windows' | 'android' | 'iphone'
const platform = ref<Platform>('windows')
const account = ref<{ username: string; number: string; online: boolean; state: string } | null>(null)
const server = ref('sip.leadomi.com'), proxy = ref(''), port = ref(5061)
const loaded = ref(false), copied = ref(''), provisioning = ref('')

async function load() {
  try {
    const res = await api.get('/business/accounts')
    account.value = (res.data.data || [])[0] || null
    server.value = res.data.server || server.value
    port.value = res.data.sip_port || 5061
    provisioning.value = res.data.provisioning_url || ''
    proxy.value = `<sip:${server.value}:${port.value};transport=tls>`
  } catch { /* keep last values */ } finally { loaded.value = true }
}
async function copy(key: string, value: string) {
  try { await navigator.clipboard.writeText(value); copied.value = key; setTimeout(() => { if (copied.value === key) copied.value = '' }, 1500) } catch { /* clipboard blocked */ }
}
const values = computed(() => [
  { key: 'username', label: 'Username', value: account.value?.username || 'leadomi' },
  { key: 'domain', label: 'SIP domain / server', value: server.value },
  { key: 'proxy', label: 'SIP proxy / server address', value: proxy.value },
  { key: 'transport', label: 'Transport', value: 'TLS' },
  { key: 'port', label: 'Port', value: String(port.value) },
])
const steps: Record<Platform, { title: string; items: string[] }[]> = {
  windows: [
    { title: 'Install', items: ['Download Linphone for Windows from linphone.org and install it.', 'When Windows Firewall asks, click Allow access (Private and Public networks) — otherwise audio may be one-way.'] },
    { title: 'Add the account', items: ['Open Linphone → Use a SIP account (or ☰ → Assistant → Use a SIP account).', 'Enter the Username, SIP domain and Password, and choose Transport: TLS. Click Use / Log in.'] },
    { title: 'Server address (important)', items: ['☰ → Settings → SIP Accounts → Edit your account.', 'Set SIP proxy / Server address to the value above (with < >).', 'Turn Outbound proxy ON and Registration ON. Save.'] },
    { title: 'Encryption and audio', items: ['Settings → Calls (or Security): Media encryption = SRTP, and Encryption is mandatory = ON.', 'Settings → Audio → Codecs: make sure PCMU and PCMA are enabled.'] },
    { title: 'Network and messages', items: ['Settings → Network: turn OFF “Allow IPv6”, then restart Linphone. The server is IPv4 only; with IPv6 on, calls connect with no sound.', 'Settings → Chat: turn OFF “Use CPIM in basic conversations” if present.'] },
  ],
  android: [
    { title: 'Install', items: ['Install Linphone from the Google Play Store and open it.'] },
    { title: 'Add the account', items: ['Choose Use a SIP account.', 'Enter the Username, Password and SIP domain; set Transport to TLS. Tap Login.'] },
    { title: 'Server address (important)', items: ['Settings → Accounts → your account (or Account settings).', 'Set SIP proxy / Server address to the value above and turn Outbound proxy ON.'] },
    { title: 'Encryption and audio', items: ['Settings → Calls: Media encryption = SRTP, Encryption mandatory = ON.', 'Settings → Audio: make sure PCMU and PCMA codecs are enabled.'] },
    { title: 'Keep it ringing', items: ['Settings → Network: turn OFF “Allow IPv6” (mobile data often uses IPv6; the server is IPv4 only).', 'Android Settings → Apps → Linphone → Battery: set to Unrestricted (no battery optimisation).', 'Allow notifications and “Display over other apps” so incoming calls show.', 'Settings → Chat: turn OFF “Use CPIM in basic conversations”.'] },
  ],
  iphone: [
    { title: 'Install', items: ['Install Linphone from the App Store and open it. Allow microphone and notifications.'] },
    { title: 'Add the account', items: ['Choose Use a SIP account.', 'Enter the Username, Password and SIP domain; set Transport to TLS. Tap Login.'] },
    { title: 'Server address (important)', items: ['Settings → Account → your account.', 'Set SIP proxy / Server address to the value above and turn Outbound proxy ON.'] },
    { title: 'Encryption and audio', items: ['Settings → Calls: Media encryption = SRTP, Encryption mandatory = ON.', 'Settings → Audio: make sure PCMU and PCMA codecs are enabled.'] },
    { title: 'Keep it ringing', items: ['Settings → Network: turn OFF “Allow IPv6” (the server is IPv4 only).', 'iPhone apps sleep in the background: keep Linphone open (or in recent apps) when expecting calls.', 'Settings → Chat: turn OFF “Use CPIM in basic conversations”.'] },
  ],
}
const tabs: { id: Platform; label: string; icon: typeof Monitor }[] = [
  { id: 'windows', label: 'Windows', icon: Monitor }, { id: 'android', label: 'Android', icon: Smartphone }, { id: 'iphone', label: 'iPhone', icon: Tablet },
]
let timer: ReturnType<typeof setInterval>
onMounted(() => { load(); timer = setInterval(load, 10000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold">Phone setup</h1>
        <p class="text-sm text-gray-500 mt-2">Connect Linphone to your business number. Only one device can be signed in at a time.</p>
      </div>
      <div v-if="loaded" :class="['rounded-lg border px-4 py-2 text-sm', account?.online ? 'border-green-600 text-green-400' : 'border-amber-500 text-amber-400']" role="status">
        <span class="font-semibold">{{ account?.online ? '● Phone connected' : '○ Phone not connected' }}</span>
        <span class="block text-xs opacity-80">{{ account?.online ? 'Calls and texts will reach Linphone.' : 'Follow the steps below. This updates automatically.' }}</span>
      </div>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-lg p-5 space-y-3">
      <h2 class="font-semibold">Your settings</h2>
      <div class="grid sm:grid-cols-2 gap-3">
        <div v-for="v in values" :key="v.key" class="flex items-center justify-between gap-3 rounded border border-gray-200 dark:border-gray-700 px-3 py-2">
          <div class="min-w-0"><p class="text-xs text-gray-500">{{ v.label }}</p><p class="font-mono text-sm break-all">{{ v.value }}</p></div>
          <button type="button" @click="copy(v.key, v.value)" class="shrink-0 border rounded px-2 py-1 text-xs inline-flex items-center gap-1" :aria-label="'Copy ' + v.label">
            <component :is="copied === v.key ? Check : Copy" class="h-3.5 w-3.5" />{{ copied === v.key ? 'Copied' : 'Copy' }}
          </button>
        </div>
        <div class="rounded border border-gray-200 dark:border-gray-700 px-3 py-2 sm:col-span-2">
          <p class="text-xs text-gray-500">Password</p>
          <p class="text-sm">The phone password set as <span class="font-mono">GOSIP_PHONE_PASSWORD</span> in Coolify (leadomi-sip → Environment Variables). It is never shown here.</p>
        </div>
      </div>
      <div class="flex flex-wrap gap-2 text-xs">
        <span class="border rounded-full px-2 py-0.5">Outbound proxy: ON</span>
        <span class="border rounded-full px-2 py-0.5">Media encryption: SRTP (mandatory)</span>
        <span class="border rounded-full px-2 py-0.5">Codecs: PCMU + PCMA</span>
        <span class="border rounded-full px-2 py-0.5">CPIM: OFF</span>
        <span class="border rounded-full px-2 py-0.5">Allow IPv6: OFF</span>
      </div>
    </div>

    <div v-if="provisioning" class="bg-white dark:bg-gray-800 rounded-lg p-5 space-y-3 border border-primary/40">
      <h2 class="font-semibold">Easiest: automatic setup link</h2>
      <p class="text-sm text-gray-500">Linphone fills in the server, port, TLS and SRTP for you. Use this when the port is not 5061, because the Linphone sign-in form cannot set a port.</p>
      <div class="flex items-center justify-between gap-3 rounded border border-gray-200 dark:border-gray-700 px-3 py-2">
        <p class="font-mono text-xs break-all min-w-0">{{ provisioning }}</p>
        <button type="button" @click="copy('provisioning', provisioning)" class="shrink-0 border rounded px-2 py-1 text-xs inline-flex items-center gap-1" aria-label="Copy setup link">
          <component :is="copied === 'provisioning' ? Check : Copy" class="h-3.5 w-3.5" />{{ copied === 'provisioning' ? 'Copied' : 'Copy' }}
        </button>
      </div>
      <ol class="list-decimal pl-5 text-sm space-y-1">
        <li>Linphone start screen → <b>Fetch remote configuration</b> (Windows: ☰ → Assistant → Fetch remote configuration).</li>
        <li>Paste the link and confirm. Linphone restarts with the account added.</li>
        <li>When it asks for the password, enter the phone password. Done.</li>
      </ol>
      <p class="text-xs text-gray-500">The link holds no password. Keep it private anyway.</p>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-lg p-5 space-y-4">
      <div class="flex flex-wrap gap-2" role="tablist" aria-label="Device">
        <button v-for="t in tabs" :key="t.id" type="button" role="tab" :aria-selected="platform === t.id" @click="platform = t.id"
          :class="['rounded-lg px-4 py-2 text-sm inline-flex items-center gap-2 border', platform === t.id ? 'bg-primary text-white border-primary' : 'border-gray-300 dark:border-gray-600']">
          <component :is="t.icon" class="h-4 w-4" />{{ t.label }}
        </button>
      </div>
      <ol class="space-y-4">
        <li v-for="(step, i) in steps[platform]" :key="step.title" class="flex gap-3">
          <span class="shrink-0 h-7 w-7 rounded-full bg-primary/20 text-primary flex items-center justify-center text-sm font-semibold">{{ i + 1 }}</span>
          <div><p class="font-medium">{{ step.title }}</p><ul class="list-disc pl-5 text-sm text-gray-500 dark:text-gray-400 space-y-1 mt-1"><li v-for="item in step.items" :key="item">{{ item }}</li></ul></div>
        </li>
      </ol>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-lg p-5 space-y-3">
      <h2 class="font-semibold">Using it</h2>
      <ul class="list-disc pl-5 text-sm space-y-1">
        <li>Calls: dial with the country code, e.g. <span class="font-mono">+447700900123</span>. Callers see {{ account?.number || 'your business number' }}.</li>
        <li>Texts: start a chat with <span class="font-mono">sip:+447700900123@{{ server }}</span>.</li>
      </ul>
      <h2 class="font-semibold pt-2">If it doesn't work</h2>
      <ul class="list-disc pl-5 text-sm space-y-1">
        <li><b>Registration failed / red dot:</b> check the password (no spaces) and the server address including <span class="font-mono">:{{ port }};transport=tls</span>.</li>
        <li><b>Stuck on “In progress”:</b> your network may block port {{ port }}. Try a mobile hotspot, or turn off VPNs such as Cloudflare WARP.</li>
        <li><b>“User cannot accept your call”:</b> media encryption isn't SRTP, or PCMU/PCMA codecs are disabled.</li>
        <li><b>Call drops after ~30 seconds:</b> Outbound proxy is off — turn it on.</li>
        <li><b>Call connects but no sound at all:</b> open the Dashboard and run the Setup check. If “Call audio address” is red, the server's public IP changed: update <span class="font-mono">PBX_PUBLIC_IP</span> in Coolify and redeploy.</li>
        <li><b>One-way audio:</b> turn OFF “Allow IPv6” in Linphone, check the microphone in Linphone → Settings → Audio and Windows microphone privacy, allow Linphone through the firewall, and turn off VPNs.</li>
        <li><b>Port is not 5061:</b> the sign-in form cannot set a port. Use the automatic setup link above, or put the full server address (with <span class="font-mono">&lt; &gt;</span>) in Proxy server URL with Outgoing proxy ON.</li>
      </ul>
    </div>
  </div>
</template>
