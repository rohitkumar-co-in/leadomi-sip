<script setup lang="ts">
import {ref,onMounted,watch,computed} from 'vue'
import {useRoute} from 'vue-router'
import api from '@/api/client'
import {dateTime,statusLabel,toneClass} from '@/utils/display'
import HistoryDeleteDialog from '@/components/HistoryDeleteDialog.vue'
import {useAuthStore} from '@/stores/auth'
import BulkHistoryDeleteDialog from '@/components/BulkHistoryDeleteDialog.vue'
import type {HistoryRecord} from '@/utils/bulk-history'
const auth=useAuthStore()
const deleting=ref<Activity|null>(null)
async function historyDeleted(){deleting.value=null;if(data.value.length===1&&offset.value)offset.value=Math.max(0,offset.value-50);await load()}
const route=useRoute()
const isCallHistory=computed(()=>route.path==='/calls')
const selected=ref<string[]>([]),bulkDeleting=ref<HistoryRecord[]|null>(null)
const allVisibleSelected=computed(()=>data.value.length>0&&data.value.every(item=>selected.value.includes(`${item.kind}-${item.id}`)))
function selectVisible(){selected.value=allVisibleSelected.value?[]:data.value.map(item=>`${item.kind}-${item.id}`)}
function deleteSelected(){bulkDeleting.value=data.value.filter(item=>selected.value.includes(`${item.kind}-${item.id}`)).map(item=>({...item,kind:item.kind==='call'?'call':'sms'}))}
async function bulkChanged(){await load();if(!data.value.length&&offset.value){offset.value=Math.max(0,offset.value-50);await load()}}
interface Activity {kind:string;id:number;direction:string;from_number:string;to_number:string;status:string;occurred_at:string;duration:number;body:string;actor:string}
const data=ref<Activity[]>([]),loading=ref(false),error=ref(''),offset=ref(0),kind=ref(route.path==='/calls'?'call':'')
let activityRequest=0
async function load(){const request=++activityRequest;selected.value=[];loading.value=true;error.value='';try{const response=await api.get('/business/activity',{params:{offset:offset.value,kind:isCallHistory.value?'call':kind.value}});if(request===activityRequest)data.value=response.data.data}catch{if(request===activityRequest)error.value='Activity could not be loaded. Try again.'}finally{if(request===activityRequest)loading.value=false}}

watch(()=>route.path,()=>{kind.value=route.path==='/calls'?'call':'';offset.value=0;load()})
onMounted(load)
</script>
<template><div class="space-y-6 page-enter">
 <div class="flex flex-wrap items-start justify-between gap-4"><div><h1 class="text-2xl font-semibold">{{route.path==='/calls'?'Call history':'Activity log'}}</h1><p class="text-gray-500 text-sm mt-2">{{isCallHistory?'Incoming and outgoing calls, with numbers, account, status, and time.':'Every call and text on your number, newest first.'}}</p></div><button class="border rounded-lg px-4 py-2" @click="load" :disabled="loading">Refresh</button></div>
 <div class="glass rounded-lg p-4 flex flex-wrap gap-4 items-center"><label v-if="!isCallHistory">Type <select v-model="kind" @change="offset=0;load()" class="ml-2 border rounded p-2"><option value="">Calls and SMS</option><option value="call">Calls</option><option value="sms">SMS</option></select></label><p class="text-sm text-gray-500">Times are shown in your device’s timezone.</p></div>
 <div v-if="auth.isAdmin&&data.length" class="flex flex-wrap gap-3 items-center"><label class="flex gap-2 items-center text-sm"><input type="checkbox" :checked="allVisibleSelected" :disabled="loading" @change="selectVisible" />Select all on this page</label><button class="text-red-600 underline text-sm" :disabled="!selected.length||loading" @click="deleteSelected">Delete selected ({{selected.length}})</button></div><p v-if="error" role="alert" class="text-red-600">{{error}}</p>
 <div class="glass rounded-lg overflow-x-auto"><table class="responsive-table w-full text-sm text-left"><thead><tr><th v-if="auth.isAdmin" class="p-4">Select</th><th class="p-4">When</th><th class="p-4">Type</th><th class="p-4">From → To</th><th class="p-4">Result / message</th></tr></thead><tbody><tr v-for="item in data" :key="`${item.kind}-${item.id}`" class="border-t"><td v-if="auth.isAdmin" data-label="Select" class="p-4"><input v-model="selected" type="checkbox" :value="`${item.kind}-${item.id}`" :disabled="loading" :aria-label="'Select '+item.kind+' record '+item.id" /></td><td data-label="When" class="p-4 whitespace-nowrap">{{dateTime(item.occurred_at)}}</td><td data-label="Type" class="p-4 whitespace-nowrap"><span class="font-medium">{{item.direction==='inbound'?'↙ Incoming':'↗ Outgoing'}} {{item.kind==='call'?'call':'SMS'}}</span><p v-if="item.actor" class="text-gray-500 text-xs mt-1">{{item.actor}}</p></td><td data-label="From → To" class="p-4 font-mono whitespace-nowrap">{{item.from_number}}<p class="text-gray-500 mt-1">→ {{item.to_number}}</p></td><td data-label="Result / message" class="p-4"><span :class="['inline-block border rounded-full px-2 py-0.5 text-xs font-medium',toneClass[statusLabel(item.status).tone]]">{{statusLabel(item.status).label}}</span><p v-if="item.kind==='call'" class="text-gray-500 mt-1">{{item.duration?item.duration+' s':'—'}}</p><p v-else class="text-gray-400 mt-1 whitespace-pre-wrap break-words max-w-md">{{item.body}}</p><button v-if="auth.isAdmin" type="button" class="text-xs text-gray-500 hover:text-red-500 underline mt-2" @click="deleting=item">Delete</button></td></tr><tr v-if="!data.length"><td :colspan="auth.isAdmin?5:4" class="p-8 text-gray-500">{{loading?'Loading activity…':'No activity on this page.'}}</td></tr></tbody></table></div>
 <div class="flex flex-wrap gap-3 items-center justify-between"><button class="border px-4 py-2 rounded" :disabled="!offset||loading" @click="offset=Math.max(0,offset-50);load()">Previous</button><span class="text-sm text-gray-500">{{data.length?'Records '+(offset+1)+'–'+(offset+data.length):'No records'}}</span><button class="border px-4 py-2 rounded" :disabled="data.length<50||loading" @click="offset+=50;load()">Next</button></div>
 <HistoryDeleteDialog v-if="deleting" :kind="deleting.kind==='call'?'call':'sms'" :id="deleting.id" :from="deleting.from_number" :to="deleting.to_number" @close="deleting=null" @deleted="historyDeleted" />
 <BulkHistoryDeleteDialog v-if="bulkDeleting" :records="bulkDeleting" @close="bulkDeleting=null" @changed="bulkChanged" />
</div></template>
