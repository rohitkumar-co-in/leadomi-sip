export function list<T>(value:unknown):T[]{return Array.isArray(value)?value as T[]:[]}
export function dateTime(value?:string):string{
 if(!value)return '—'
 const normalized=/[zZ]|[+-]\d\d:\d\d$/.test(value)?value:value.replace(' ','T')+'Z'
 const date=new Date(normalized)
 return Number.isNaN(date.getTime())?'—':date.toLocaleString('en-GB',{day:'numeric',month:'short',year:'numeric',hour:'2-digit',minute:'2-digit'})
}

export type Tone = 'good' | 'warn' | 'bad' | 'muted' | 'test'
const statusLabels: Record<string, [string, Tone]> = {
  completed: ['Answered', 'good'], answered: ['Answered', 'good'], 'in-progress': ['In progress', 'warn'],
  ringing: ['Ringing', 'warn'], queued: ['Queued', 'warn'], initiated: ['Starting', 'warn'],
  'no-answer': ['Missed', 'bad'], missed: ['Missed', 'bad'], busy: ['Busy', 'bad'], failed: ['Failed', 'bad'],
  canceled: ['Cancelled', 'muted'], blocked: ['Blocked', 'muted'], voicemail: ['Voicemail', 'warn'],
  received: ['Received', 'good'], delivered: ['Delivered', 'good'], sent: ['Sent', 'good'], read: ['Read', 'good'],
  undelivered: ['Not delivered', 'bad'], sending: ['Sending', 'warn'], accepted: ['Sending', 'warn'],
  simulated: ['Test (not sent)', 'test'],
}
// Plain-English label and colour for call/SMS statuses.
export function statusLabel(status?: string): { label: string; tone: Tone } {
  const [label, tone] = statusLabels[(status || '').toLowerCase()] || [status || 'Unknown', 'muted' as Tone]
  return { label, tone }
}
export const toneClass: Record<Tone, string> = {
  good: 'border-green-600/60 text-green-400', warn: 'border-amber-500/60 text-amber-400',
  bad: 'border-red-500/60 text-red-400', muted: 'border-gray-500/60 text-gray-400', test: 'border-purple-500/60 text-purple-300',
}
const auditLabels: Record<string, string> = {
  env_phone_user: 'Phone user set up from server settings', provision: 'Phone user provisioned',
  edit_user: 'Phone user edited', disable: 'Phone user disabled', delete: 'Phone user deleted',
  select_phone_number: 'Phone number changed',
}
export function auditLabel(action: string): string { return auditLabels[action] || action.replace(/_/g, ' ') }
