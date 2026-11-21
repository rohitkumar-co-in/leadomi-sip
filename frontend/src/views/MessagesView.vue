<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { MessageSquare, Send, RefreshCw, ArrowLeft } from 'lucide-vue-next'
import api from '@/api/client'
import {dateTime} from '@/utils/display'
import HistoryDeleteDialog from '@/components/HistoryDeleteDialog.vue'
import {useAuthStore} from '@/stores/auth'
const auth=useAuthStore()
const deleting=ref<Message|null>(null)
import BulkHistoryDeleteDialog from '@/components/BulkHistoryDeleteDialog.vue'
import type {HistoryRecord} from '@/utils/bulk-history'
import {collectConversationHistory} from '@/utils/bulk-history'
const selectedMessages=ref<number[]>([]),bulkDeleting=ref<HistoryRecord[]|null>(null)
const selectedConversations=ref<string[]>([]),preparingConversations=ref(false),deletingConversations=ref<string[]>([]),deletionBusinessNumber=ref('')
const allConversationsSelected=computed(()=>conversations.value.length>0&&conversations.value.every(conv=>selectedConversations.value.includes(conv.phone_number)))
function selectConversations(){selectedConversations.value=allConversationsSelected.value?[]:conversations.value.map(conv=>conv.phone_number)}
async function deleteConversations(numbers:string[]){
 if(preparingConversations.value||!numbers.length)return
 const did=selectedDID.value
 const selected=[...new Set(numbers)]
 preparingConversations.value=true;error.value=null
 try{
  const records=await collectConversationHistory(selected,did,async(number,filter,offset,limit)=>(await api.get(`/messages/conversation/${encodeURIComponent(number)}`,{params:{did_id:filter,offset,limit}})).data)
  if(!records.length){error.value='These conversations have no stored messages to delete.';return}
  deletingConversations.value=selected
  deletionBusinessNumber.value=did?(dids.value.find(item=>item.id===did)?.phone_number||'Selected business number'):'All business numbers'
  bulkDeleting.value=records
 }catch{error.value='Could not load the complete conversations. No messages were deleted. Try again.'}
 finally{preparingConversations.value=false}
}
const allLoadedSelected=computed(()=>messages.value.length>0&&messages.value.every(message=>selectedMessages.value.includes(message.id)))
function selectLoaded(){selectedMessages.value=allLoadedSelected.value?[]:messages.value.map(message=>message.id)}
function deleteSelectedMessages(){deletingConversations.value=[];bulkDeleting.value=messages.value.filter(message=>selectedMessages.value.includes(message.id)).map(message=>({...message,kind:'sms'}))}
async function bulkChanged(){selectedMessages.value=[];selectedConversations.value=[];await refreshMessages();if(selectedConversation.value&&!messages.value.length&&!conversations.value.some(conv=>conv.phone_number===selectedConversation.value)){selectedConversation.value=null;messages.value=[];hasOlderMessages.value=false}}
async function messageDeleted(){deleting.value=null;await bulkChanged()}

interface Message {
  id: number
  direction: 'inbound' | 'outbound'
  from_number: string
  to_number: string
  body: string
  media_urls?: string[]
  status: string
  created_at: string
}

interface Conversation {
  phone_number: string
  message_count: number
  last_message_at: string
  unread_count: number
}

const conversations = ref<Conversation[]>([])
const messages = ref<Message[]>([])
const selectedConversation = ref<string | null>(null)
const loading = ref(true)
const sendingMessage = ref(false)
const error = ref<string | null>(null)
const newMessage = ref('')
const hasOlderMessages = ref(false)
const loadingOlder = ref(false)
let messageRequest = 0
let conversationRequest = 0

const dids = ref<{ id: number; phone_number: string; friendly_name: string }[]>([])
const selectedDID = ref<number>(0)
const sendingDID = ref<number>(0)
const recipient=ref('')
const singleNumber=computed(()=>dids.value.length===1)
watch(selectedDID,()=>{messageRequest++;selectedMessages.value=[];selectedConversations.value=[];selectedConversation.value=null;messages.value=[];hasOlderMessages.value=false;sendingDID.value=selectedDID.value;loadConversations()})

const route=useRoute()
onMounted(async () => {
  await loadDIDs();await loadConversations()
  // Opened from the Dialer: jump to that number's conversation or start one.
  const to=typeof route.query.to==='string'?route.query.to:''
  if(/^\+[1-9]\d{7,14}$/.test(to)){recipient.value=to;if(conversations.value.some(conv=>conv.phone_number===to)){selectedConversation.value=to;await loadMessages(to)}}
})

async function loadDIDs() {
  try {
    const response = await api.get('/dids')
    dids.value = response.data.data || []
    // A single business number needs no picker: use it for viewing and sending.
    if (dids.value.length === 1) { selectedDID.value = dids.value[0].id; sendingDID.value = dids.value[0].id }
  } catch {
    console.error('Failed to load DIDs')
  }
}

async function loadConversations() {
  const request = ++conversationRequest
  loading.value = true
  error.value = null
  try {
    const response = await api.get('/messages/conversations',{params:selectedDID.value?{did_id:selectedDID.value}:{}})
    if (request === conversationRequest) {
      conversations.value = response.data.data || []
      selectedConversations.value=selectedConversations.value.filter(number=>conversations.value.some(conv=>conv.phone_number===number))
    }
  } catch {
    if (request === conversationRequest) error.value = 'Failed to load conversations'
  } finally {
    if (request === conversationRequest) loading.value = false
  }
}

async function loadMessages(phoneNumber: string, older = false) {
  if (older && loadingOlder.value) return
  const request = ++messageRequest
  const did = selectedDID.value
  const offset = older ? messages.value.length : 0
  if (older) loadingOlder.value = true
  else { selectedMessages.value=[];loading.value = true; loadingOlder.value = false; messages.value = []; hasOlderMessages.value = false }
  error.value = null
  selectedConversation.value = phoneNumber
  try {
    const response = await api.get(`/messages/conversation/${encodeURIComponent(phoneNumber)}`,{params:{did_id:did,limit:50,offset}})
    if (request !== messageRequest) return
    messages.value = older ? [...messages.value, ...(response.data.data || [])] : (response.data.data || [])
    hasOlderMessages.value = response.data.has_more === true
  } catch {
    if (request === messageRequest) error.value = 'Failed to load messages'
  } finally {
    if (request === messageRequest) { loading.value = false; loadingOlder.value = false }
  }
}

async function refreshMessages() {
  await loadConversations()
  if (selectedConversation.value) await loadMessages(selectedConversation.value)
}

async function sendMessage() {
  if (!newMessage.value.trim() || !selectedConversation.value || !sendingDID.value) return

  sendingMessage.value = true
  try {
    await api.post('/messages', {
      to_number: selectedConversation.value,
      did_id: sendingDID.value,
      body: newMessage.value
    })
    newMessage.value = ''
    await loadMessages(selectedConversation.value);await loadConversations()
  } catch (err: unknown) {
    const apiError = err as { response?: { data?: { error?: { message?: string } } } }
    error.value = apiError.response?.data?.error?.message || 'Failed to send message'
  } finally {
    sendingMessage.value = false
  }
}

function formatTime(value:string){return dateTime(value)}

function formatPhoneNumber(phone: string): string {
  if (phone.startsWith('+1') && phone.length === 12) {
    return `(${phone.slice(2, 5)}) ${phone.slice(5, 8)}-${phone.slice(8)}`
  }
  return phone
}

const sortedMessages = computed(() => {
  return [...messages.value].sort((a, b) =>
    new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  )
})
</script>

<template>
  <div class="messages-page">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Messages</h1>
      <button
        @click="refreshMessages" aria-label="Refresh messages" :disabled="preparingConversations"
        class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md"
      >
        <RefreshCw class="h-4 w-4" />
      </button>
    </div>

    <div v-if="error" class="mb-4 bg-destructive/10 text-destructive px-4 py-3 rounded-md">
      {{ error }}
    </div>

    <p v-if="singleNumber" class="text-sm text-gray-500 -mt-2">SMS for {{dids[0].phone_number}}</p><div class="message-tools"><label v-if="!singleNumber" class="text-sm">Business number<select v-model="selectedDID" :disabled="preparingConversations" class="ml-2 border rounded p-2"><option :value="0">All business numbers</option><option v-for="did in dids" :key="did.id" :value="did.id">{{did.phone_number}}</option></select></label><form @submit.prevent="/^\+[1-9][0-9]{7,14}$/.test(recipient)&&loadMessages(recipient)" class="new-conversation-form"><input v-model="recipient" aria-label="New conversation number" placeholder="Number with country code, e.g. +447700900000" pattern="\+[1-9][0-9]{7,14}" required class="border rounded p-2 text-sm" /><button :disabled="preparingConversations" class="border rounded px-4 text-sm whitespace-nowrap">New conversation</button></form></div>
    <div class="messages-workspace flex bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
      <!-- Conversations List -->
      <div
        :class="[
          'conversation-list w-full md:w-72 lg:w-80 shrink-0 min-h-0 border-r border-gray-200 dark:border-gray-700 flex flex-col',
          selectedConversation ? 'hidden md:flex' : 'flex'
        ]"
      >
        <div class="p-4 border-b border-gray-200 dark:border-gray-700">
          <h2 class="font-medium text-gray-900 dark:text-white">Conversations</h2>
          <div v-if="auth.isAdmin&&conversations.length" class="mt-3 space-y-3"><label class="flex gap-2 items-center text-sm"><input type="checkbox" :checked="allConversationsSelected" :disabled="loading||preparingConversations" @change="selectConversations" />Select all conversations</label><button type="button" class="text-sm text-red-600 underline" :disabled="!selectedConversations.length||loading||preparingConversations" @click="deleteConversations(selectedConversations)">Delete selected conversations ({{selectedConversations.length}})</button></div>
          <p v-if="preparingConversations" role="status" class="mt-3 text-sm">Loading complete conversation history…</p>
        </div>

        <div v-if="loading && !selectedConversation" class="p-4 text-gray-500">
          Loading...
        </div>

        <div v-else class="flex-1 overflow-y-auto">
          <div
            v-for="conv in conversations"
            :key="conv.phone_number"
            :class="[
              'w-full p-4 text-left hover:bg-gray-50 dark:hover:bg-gray-700/50 border-b border-gray-100 dark:border-gray-700',
              selectedConversation === conv.phone_number && 'bg-primary/5'
            ]"
          >
            <div v-if="auth.isAdmin" class="flex flex-wrap gap-3 items-center mb-3"><label class="flex gap-2 items-center text-xs"><input v-model="selectedConversations" type="checkbox" :value="conv.phone_number" :disabled="preparingConversations" :aria-label="'Select conversation with '+conv.phone_number" />Select conversation</label><button type="button" class="text-xs text-red-600 underline" :disabled="preparingConversations||loading" @click="deleteConversations([conv.phone_number])">Delete conversation</button></div>
            <button type="button" class="w-full text-left" :disabled="preparingConversations" @click="loadMessages(conv.phone_number)">
            <div class="flex flex-wrap gap-1 items-center justify-between">
              <span class="font-medium text-gray-900 dark:text-white">
                {{ formatPhoneNumber(conv.phone_number) }}
              </span>
              <span class="text-xs text-gray-500">
                {{ formatTime(conv.last_message_at) }}
              </span>
            </div>
            <div class="flex items-center justify-between mt-1">
              <p class="text-sm text-gray-500 truncate max-w-[200px]">
                {{ conv.message_count }} messages
              </p>
              <span
                v-if="conv.unread_count > 0"
                class="inline-flex items-center justify-center w-5 h-5 text-xs font-medium text-white bg-primary rounded-full"
              >
                {{ conv.unread_count }}
              </span>
            </div>
            </button>
          </div>

          <div v-if="conversations.length === 0" class="p-4 text-center text-gray-500">
            No conversations yet
          </div>
        </div>
      </div>

      <!-- Messages View -->
      <div
        :class="[
          'flex-1 min-w-0 min-h-0 flex flex-col',
          !selectedConversation ? 'hidden md:flex' : 'flex'
        ]"
      >
        <div v-if="selectedConversation" class="flex flex-col h-full min-h-0">
          <!-- Header -->
          <div class="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center">
            <button
              @click="selectedConversation = null" aria-label="Back to conversations"
              class="md:hidden mr-3 p-3 text-gray-500 hover:text-gray-700"
            >
              <ArrowLeft class="h-5 w-5" />
            </button>
            <MessageSquare class="h-5 w-5 text-gray-400 mr-2" />
            <span class="font-medium text-gray-900 dark:text-white">
              {{ formatPhoneNumber(selectedConversation) }}
            </span>
          </div>

          <!-- Messages -->
          <div v-if="auth.isAdmin&&messages.length" class="p-3 border-b flex flex-wrap gap-3 items-center"><label class="flex gap-2 items-center text-sm"><input type="checkbox" :checked="allLoadedSelected" @change="selectLoaded" />Select all loaded messages</label><button type="button" class="text-sm text-red-600 underline" :disabled="!selectedMessages.length||loading||loadingOlder" @click="deleteSelectedMessages">Delete selected ({{selectedMessages.length}})</button></div>
          <div class="flex-1 min-h-0 overflow-y-auto p-4 space-y-4">
            <div v-if="hasOlderMessages" class="text-center"><button class="border rounded px-4 py-2 text-sm" :disabled="loadingOlder" @click="loadMessages(selectedConversation, true)">{{loadingOlder?'Loading older messages…':'Load older messages'}}</button></div>
            <div v-if="loading" class="text-center text-gray-500">Loading...</div>

            <div
              v-for="message in sortedMessages"
              :key="message.id"
              :class="[
                'flex',
                message.direction === 'outbound' ? 'justify-end' : 'justify-start'
              ]"
            >
              <div
                :class="[
                  'message-bubble rounded-lg px-4 py-2',
                  message.direction === 'outbound'
                    ? 'bg-primary text-white'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white'
                ]"
              >
                <p class="whitespace-pre-wrap break-words">{{ message.body }}</p>
                <div
                  v-if="message.media_urls?.length"
                  class="mt-2 space-y-2"
                >
                  <img
                    v-for="(url, index) in message.media_urls"
                    :key="index"
                    :src="url"
                    class="max-w-full rounded"
                    alt="Media attachment"
                  />
                </div>
                <p
                  :class="[
                    'text-xs mt-1',
                    message.direction === 'outbound' ? 'text-white/70' : 'text-gray-500'
                  ]"
                >
                  {{ formatTime(message.created_at) }} · {{message.status}}<br />{{message.from_number}} → {{message.to_number}}
                </p>
                <div v-if="auth.isAdmin" class="flex gap-3 items-center mt-2"><label class="flex gap-2 items-center text-xs"><input v-model="selectedMessages" type="checkbox" :value="message.id" :aria-label="'Select message '+message.id+' for deletion'" />Select</label><button type="button" class="text-xs underline" :aria-label="'Delete message from '+message.from_number+' to '+message.to_number" @click="deleting=message">Delete</button></div>
              </div>
            </div>
          </div>

          <!-- Input -->
          <div class="p-4 border-t border-gray-200 dark:border-gray-700">
            <form @submit.prevent="sendMessage" :class="['message-composer',{'single-number':singleNumber}]">
              <select v-if="!singleNumber"
                v-model="sendingDID" aria-label="Send from business number"
                class="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white text-sm"
              >
                <option :value="0" disabled>Select sending number</option>
                <option v-for="did in dids" :key="did.id" :value="did.id">
                  {{ did.friendly_name || formatPhoneNumber(did.phone_number) }}
                </option>
              </select>
              <input
                v-model="newMessage" aria-label="Message text"
                type="text"
                placeholder="Type a message..."
                class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white"
              />
              <button
                type="submit" aria-label="Send message"
                :disabled="sendingMessage || !newMessage.trim() || !sendingDID"
                class="px-4 py-2 bg-primary text-white rounded-md hover:bg-primary/90 disabled:opacity-50"
              >
                <Send class="h-4 w-4" />
              </button>
            </form>
          </div>
        </div>

        <!-- Empty State -->
        <div v-else class="flex-1 flex items-center justify-center text-gray-500">
          <div class="text-center">
            <MessageSquare class="h-12 w-12 mx-auto mb-4 text-gray-300" />
            <p>Select a conversation to view messages</p>
          </div>
        </div>
      </div>
    </div>
    <HistoryDeleteDialog v-if="deleting" kind="sms" :id="deleting.id" :from="deleting.from_number" :to="deleting.to_number" @close="deleting=null" @deleted="messageDeleted" />
    <BulkHistoryDeleteDialog v-if="bulkDeleting" :records="bulkDeleting" :conversations="deletingConversations" :business-number="deletionBusinessNumber" @close="bulkDeleting=null;deletingConversations=[]" @changed="bulkChanged" />
  </div>
</template>
