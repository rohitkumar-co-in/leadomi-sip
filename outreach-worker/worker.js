// Leadomi Outreach - single-file Cloudflare Worker. Needs D1 binding "DB" + secrets (see .dev.vars.example)
const DEF={quota:20,demo:"",tpl:{
 first:[
  "Hi {name}, I rang {garage} on {day} at {time} and it just rang out.{pain} Most people won't leave a voicemail, they'll just ring the next garage they find on Google. I'm Rohit, I set up an automatic text-back so those callers hang on for you instead. Can I show you how it works?",
  "Hi {name}, quick one. I called {garage} on {day} around {time} and got no answer.{pain} No complaints, you were probably under a motor. But that caller will usually book in with whoever picks up next. I can make sure every missed caller gets a text within seconds. Worth a look?",
  "Hello {name}, I tried {garage} on {day} at {time} and couldn't get through.{pain} One missed MOT or repair is easily £50 to £300 off to another garage. I'm Rohit, I set up missed-call text-back for small garages. Free for a week, no card, no contract. Shall I set it up for you?"],
 nocall:[
  "Hi {name}, when you're under a motor and the phone rings out at {garage}, most people won't leave a voicemail. They'll just ring the next garage.{pain} I'm Rohit, I set up an automatic text-back so they hang on for you instead. Free for a week, no card. Would that help?",
  "Hello {name}, quick question for {garage}. How many calls do you miss while you're busy on the tools? Each one is usually a job that goes elsewhere.{pain} I can text those callers back automatically within seconds. Free for a week, no contract. Interested?"],
 follow1:[
  "Hi {name}, Rohit again. Easiest way to see it: ring {demo}, let it ring out, and you'll get the exact text your customers would get. No links, nothing to sign up for. Free for a week, then £99 a month only if you keep it.",
  "Hi {name}, just following up about missed calls at {garage}. Don't take my word for it, ring {demo} and let it ring out. The text you get back is what your customers would see. Free week, cancel any time."],
 follow2:[
  "Last one from me, {name}. If {garage} never misses a call, ignore this. If it does, just reply YES and I'll set up a free week. Nothing to pay unless you keep it.",
  "Hi {name}, I'll leave it there. If missed calls are costing {garage} jobs, just reply YES and I'll set it up free for a week. If not, no worries, all the best."]}};
const F=["business","city","phone","website","email","instagram","facebook","rating","reviews","owner_name","call1_at","call1_result","call2_at","call2_result","last_review_date","pain_quote","status","notes","reply_text"];
const SCHEMA=[
`CREATE TABLE IF NOT EXISTS leads(id INTEGER PRIMARY KEY AUTOINCREMENT,business TEXT,city TEXT,phone TEXT UNIQUE,website TEXT,email TEXT,instagram TEXT,facebook TEXT,rating REAL,reviews INTEGER,owner_name TEXT,call1_at TEXT,call1_result TEXT,call2_at TEXT,call2_result TEXT,last_review_date TEXT,pain_quote TEXT,status TEXT DEFAULT 'New',notes TEXT,created_at TEXT,updated_at TEXT)`,
`CREATE TABLE IF NOT EXISTS messages(id INTEGER PRIMARY KEY AUTOINCREMENT,lead_id INTEGER,channel TEXT,type TEXT,body TEXT,provider TEXT,model TEXT,created_at TEXT,sent_at TEXT)`,
`CREATE TABLE IF NOT EXISTS settings(k TEXT PRIMARY KEY,v TEXT)`,
`CREATE TABLE IF NOT EXISTS auth_fail(ip TEXT PRIMARY KEY,n INTEGER,t INTEGER)`];
let ready=false;
const J=(o,s=200,h={})=>new Response(JSON.stringify(o),{status:s,headers:{"content-type":"application/json",...h}});
const enc=new TextEncoder();
async function hmac(secret,msg){const k=await crypto.subtle.importKey("raw",enc.encode(secret),{name:"HMAC",hash:"SHA-256"},false,["sign"]);const s=await crypto.subtle.sign("HMAC",k,enc.encode(msg));return [...new Uint8Array(s)].map(b=>b.toString(16).padStart(2,"0")).join("")}
function eq(a,b){if(a.length!==b.length)return false;let r=0;for(let i=0;i<a.length;i++)r|=a.charCodeAt(i)^b.charCodeAt(i);return r===0}
async function authed(req,env){const m=(req.headers.get("cookie")||"").match(/(?:^|;\s*)s=(\d+)\.([a-f0-9]+)/);if(!m||+m[1]<Date.now())return false;return eq(m[2],await hmac(env.SESSION_SECRET,m[1]))}
async function getCfg(env){const r=await env.DB.prepare("SELECT v FROM settings WHERE k='cfg'").first();let c={};try{c=JSON.parse(r.v)}catch(e){}
 const t={};for(const k in DEF.tpl)t[k]=Array.isArray((c.tpl||{})[k])&&c.tpl[k].length?c.tpl[k]:DEF.tpl[k];
 return{quota:+c.quota||DEF.quota,demo:c.demo||"",tpl:t}}
function ukParts(s){if(!s)return{day:"",time:""};const d=new Date(s+(s.length<=16?":00Z":"Z"));if(isNaN(d))return{day:"",time:""};
 const day=d.toLocaleDateString("en-GB",{weekday:"long",timeZone:"UTC"});let h=d.getUTCHours(),m=d.getUTCMinutes();const ap=h>=12?"pm":"am";h=h%12||12;return{day,time:h+(m?":"+String(m).padStart(2,"0"):"")+ap}}
function fill(t,l,demo){const p=ukParts(l.call1_at);const pain=l.pain_quote?" A recent review mentions customers struggling to reach you too.":"";
 return t.replace(/\{name\}/g,l.owner_name||"").replace(/\{garage\}/g,l.business||"").replace(/\{day\}/g,p.day).replace(/\{time\}/g,p.time).replace(/\{town\}/g,l.city||"").replace(/\{demo\}/g,demo||"this number").replace(/\{pain\}/g,pain)
  .replace(/\s+,/g,",").replace(/,\s*([.!?])/g,"$1").replace(/,\s*,/g,",").replace(/[ \t]{2,}/g," ").trim()}
async function generate(env,l,channel,type){const cfg=await getCfg(env);
 const proof=l.call1_at&&l.call1_result&&l.call1_result!=="Answered";const set=type==="first"?(proof?"first":"nocall"):type;const list=cfg.tpl[set]||cfg.tpl.first;
 const n=(await env.DB.prepare("SELECT COUNT(*) c FROM messages WHERE type=?").bind(type).first()).c;const i=n%list.length;
 let body=fill(list[i],l,cfg.demo);if(channel==="email")body="SUBJECT: "+(type==="first"?"Missed call at ":"Following up: ")+l.business+"\n\n"+body;
 return{body,provider:"Template",model:"variant "+(i+1)+" of "+list.length}}
const csvq=v=>{v=v==null?"":String(v);return /[",\n]/.test(v)?'"'+v.replace(/"/g,'""')+'"':v};
export default{async fetch(req,env){
 const u=new URL(req.url),path=u.pathname,m=req.method;
 if(!ready){await env.DB.batch(SCHEMA.map(s=>env.DB.prepare(s)));
  const have=(await env.DB.prepare("PRAGMA table_info(leads)").all()).results.map(c=>c.name);
  for(const c of ["reply_text","replied_at","pilot_at","paid_at"])if(!have.includes(c))await env.DB.prepare("ALTER TABLE leads ADD COLUMN "+c+" TEXT").run();ready=true}
 if(!path.startsWith("/api/"))return new Response(HTML,{headers:{"content-type":"text/html;charset=utf-8","cache-control":"no-store"}});
 try{
 if(path==="/api/login"&&m==="POST"){const ip=req.headers.get("cf-connecting-ip")||"x",now=Date.now();
  const f=await env.DB.prepare("SELECT n,t FROM auth_fail WHERE ip=?").bind(ip).first();
  if(f&&now-f.t<9e5&&f.n>=5)return J({error:"Too many attempts. Try again in 15 minutes."},429);
  const b=await req.json();
  if(env.APP_PASSCODE&&eq(String(b.passcode||""),env.APP_PASSCODE)){await env.DB.prepare("DELETE FROM auth_fail WHERE ip=?").bind(ip).run();
   const exp=String(now+2592e6),sig=await hmac(env.SESSION_SECRET,exp);
   return J({ok:1},200,{"set-cookie":"s="+exp+"."+sig+"; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=2592000"})}
  const n=f&&now-f.t<9e5?f.n+1:1;await env.DB.prepare("INSERT OR REPLACE INTO auth_fail(ip,n,t) VALUES(?,?,?)").bind(ip,n,now).run();
  return J({error:"Wrong passcode"},401)}
 if(!(await authed(req,env)))return J({error:"auth"},401);
 if(path==="/api/me")return J({ok:1});
 if(path==="/api/leads"&&m==="GET"){const r=await env.DB.prepare("SELECT l.*,(SELECT MAX(sent_at) FROM messages m WHERE m.lead_id=l.id) AS last_sent,(SELECT COUNT(*) FROM messages m WHERE m.lead_id=l.id AND m.sent_at IS NOT NULL) AS sent_count FROM leads l ORDER BY l.id").all();return J(r.results)}
 if(path==="/api/leads/import"&&m==="POST"){const{rows}=await req.json();const now=new Date().toISOString();let added=0;
  const st=env.DB.prepare("INSERT INTO leads("+F.join(",")+",created_at,updated_at) VALUES("+F.map(()=>"?").join(",")+",?,?) ON CONFLICT(phone) DO UPDATE SET "+["website","email","instagram","facebook","rating","reviews","owner_name","last_review_date","pain_quote","notes"].map(k=>k+"=COALESCE(excluded."+k+","+k+")").join(",")+",status=CASE WHEN leads.status='New' THEN excluded.status ELSE leads.status END,updated_at=excluded.updated_at");
  const res=await env.DB.batch(rows.filter(r=>r.business&&r.phone).map(r=>st.bind(...F.map(k=>k==="status"?(r.status||"New"):(r[k]===undefined||r[k]===""?null:r[k])),now,now)));
  res.forEach(x=>added+=x.meta.changes||0);return J({added,skipped:rows.length-added})}
 let x;
 if(x=path.match(/^\/api\/leads\/(\d+)$/)){if(m!=="PATCH")return J({error:"nope"},405);const b=await req.json();const ks=Object.keys(b).filter(k=>F.includes(k));
  if(ks.length)await env.DB.prepare("UPDATE leads SET "+ks.map(k=>k+"=?").join(",")+",updated_at=? WHERE id=?").bind(...ks.map(k=>b[k]===""?null:b[k]),new Date().toISOString(),x[1]).run();
  if(b.status){const now=new Date().toISOString(),st=b.status,id=x[1],q=(sql,...a)=>env.DB.prepare(sql).bind(...a).run();
   if(["New","Called","Messaged"].includes(st))await q("UPDATE leads SET replied_at=NULL,pilot_at=NULL,paid_at=NULL WHERE id=?",id);
   if(["Replied","Pilot","Paid"].includes(st))await q("UPDATE leads SET replied_at=COALESCE(replied_at,?) WHERE id=?",now,id);
   if(st==="Replied")await q("UPDATE leads SET pilot_at=NULL,paid_at=NULL WHERE id=?",id);
   if(st==="Pilot"||st==="Paid")await q("UPDATE leads SET pilot_at=COALESCE(pilot_at,?) WHERE id=?",now,id);
   if(st==="Pilot")await q("UPDATE leads SET paid_at=NULL WHERE id=?",id);
   if(st==="Paid")await q("UPDATE leads SET paid_at=COALESCE(paid_at,?) WHERE id=?",now,id)}
  return J({ok:1})}
 if(path==="/api/generate"&&m==="POST"){const b=await req.json();const l=await env.DB.prepare("SELECT * FROM leads WHERE id=?").bind(b.lead_id).first();if(!l)return J({error:"Lead not found"},404);
  const g=await generate(env,l,b.channel,b.type);const r=await env.DB.prepare("INSERT INTO messages(lead_id,channel,type,body,provider,model,created_at) VALUES(?,?,?,?,?,?,?)").bind(l.id,b.channel,b.type,g.body,g.provider,g.model,new Date().toISOString()).run();
  return J({...g,id:r.meta.last_row_id})}
 if(x=path.match(/^\/api\/messages\/(\d+)\/sent$/)){const b=await req.json();const mm=await env.DB.prepare("SELECT lead_id FROM messages WHERE id=?").bind(x[1]).first();
  await env.DB.prepare("UPDATE messages SET body=?,sent_at=? WHERE id=?").bind(b.body||"",new Date().toISOString(),x[1]).run();
  if(mm)await env.DB.prepare("UPDATE leads SET status='Messaged',updated_at=? WHERE id=? AND status IN('New','Called')").bind(new Date().toISOString(),mm.lead_id).run();return J({ok:1})}
 if(path==="/api/messages/sent"&&m==="GET"){const r=await env.DB.prepare("SELECT lead_id,channel,type,sent_at FROM messages WHERE sent_at IS NOT NULL ORDER BY id").all();return J(r.results)}
 if(x=path.match(/^\/api\/messages\/(\d+)\/unsent$/)){await env.DB.prepare("UPDATE messages SET sent_at=NULL WHERE id=?").bind(x[1]).run();return J({ok:1})}
 if(path==="/api/stats"){const off=parseInt(u.searchParams.get("tz")||"0")||0;
  const r=await env.DB.prepare("SELECT COUNT(*) c FROM messages WHERE sent_at IS NOT NULL AND date(sent_at,?)=date('now',?)").bind(off+" minutes",off+" minutes").first();return J({sent:r.c,quota:(await getCfg(env)).quota})}
 if(path==="/api/settings"){if(m==="POST"){const b=await req.json();const c=await getCfg(env);const v=b.reset?{quota:c.quota,demo:c.demo}:{quota:+b.quota||20,demo:String(b.demo||""),tpl:b.tpl};
  await env.DB.prepare("INSERT OR REPLACE INTO settings(k,v) VALUES('cfg',?)").bind(JSON.stringify(v)).run()}
  return J({cfg:await getCfg(env)})}
 if(path==="/api/export"){const r=await env.DB.prepare("SELECT * FROM leads ORDER BY id").all();const cols=["id",...F,"created_at","updated_at"];
  return new Response([cols.join(","),...r.results.map(l=>cols.map(c=>csvq(l[c])).join(","))].join("\n"),{headers:{"content-type":"text/csv","content-disposition":"attachment; filename=leads-export.csv"}})}
 return J({error:"Not found"},404)}
 catch(e){return J({error:e.message||"Server error"},500)}}};
const HTML=String.raw`<!doctype html><html lang="en-GB"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#0c0c0b"><title>Leadomi Outreach</title>
<link rel="preconnect" href="https://fonts.googleapis.com"><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin><link href="https://fonts.googleapis.com/css2?family=DM+Sans:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&family=Syne:wght@700;800&display=swap" rel="stylesheet">
<style>
:root{color-scheme:dark;--bg:#0c0c0b;--c:#141413;--c2:#1c1c1a;--t:#f0ede6;--m:#a09c92;--mm:#5c584f;--b:#2a2a27;--b2:#3d3d38;--a:#e85d04;--a2:#faa307;--r:#e34234;--g:#4ade80;
--sans:'DM Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;--head:'Syne',var(--sans);--mono:'JetBrains Mono',ui-monospace,monospace}
*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;font:14px/1.5 var(--sans);background:radial-gradient(900px 420px at 85% -10%,#e85d0412,transparent 60%),var(--bg);color:var(--t);min-height:100vh}
header{position:sticky;top:0;z-index:5;background:#0c0c0bcc;backdrop-filter:blur(14px);border-bottom:1px solid var(--b);padding:12px 18px;display:flex;gap:8px;flex-wrap:wrap;align-items:center}
h1{font:800 19px/1 var(--head);margin:0 auto 0 0;letter-spacing:-.02em;display:flex;align-items:center;gap:10px}
h1:before{content:"L";display:grid;place-items:center;width:30px;height:30px;border-radius:9px;background:linear-gradient(135deg,var(--a),var(--a2));color:#fff;font:800 16px var(--head);box-shadow:0 0 22px #e85d0455}
h1 .gt{background:linear-gradient(135deg,var(--a),var(--a2));-webkit-background-clip:text;background-clip:text;-webkit-text-fill-color:transparent}h1 .dim{color:var(--m);font-weight:700}
.hd{font:800 18px var(--head);letter-spacing:-.02em}
.pill{background:#14141399;border:1px solid var(--b);border-radius:99px;padding:5px 13px;font:600 12.5px var(--mono);color:var(--a2)}
button,select,input,textarea{font:inherit;color:var(--t);background:var(--c2);border:1px solid var(--b);border-radius:9px;padding:7px 11px;outline:none}
input,select,textarea{background:var(--bg)}
input:focus,select:focus,textarea:focus{border-color:var(--a);box-shadow:0 0 0 3px #e85d0426}
select option{background:var(--c);color:var(--t)}
button{cursor:pointer;font-weight:500;transition:.15s}button:hover{border-color:var(--a);color:#fff}
button.p{border:0;color:#fff;font-weight:600;background-size:200% 100%;background-image:linear-gradient(110deg,#e85d04 0%,#e85d04 40%,#faa307 50%,#e85d04 60%,#e85d04 100%);animation:shimmer 3s linear infinite;box-shadow:0 0 18px #e85d0433}
@keyframes shimmer{0%{background-position:-200% 0}100%{background-position:200% 0}}
button.s{padding:2px 9px;font-size:12px;border-radius:7px}button:disabled{opacity:.4;cursor:not-allowed}
#pbw{height:2px;background:var(--b)}#pb{height:2px;width:0;background:linear-gradient(90deg,var(--a),var(--a2));transition:width .4s;box-shadow:0 0 10px #e85d04}
.bar{display:flex;gap:8px;padding:16px 18px 8px;max-width:1500px;margin:auto;flex-wrap:wrap}.bar input{flex:1;min-width:200px}
.chips{display:flex;gap:6px;flex-wrap:wrap;padding:4px 18px 0;max-width:1500px;margin:auto}.chips button{border-radius:99px;padding:3px 12px;font-size:12.5px;color:var(--m)}.chips button.on{background:var(--a);color:#fff;border-color:var(--a)}
.cnt{padding:2px 18px 8px;max-width:1500px;margin:auto}
#list{padding:6px 18px 60px;display:grid;gap:14px;grid-template-columns:repeat(auto-fill,minmax(350px,1fr));max-width:1500px;margin:auto}
.card{background:#141413b3;backdrop-filter:blur(12px);border:1px solid #ffffff12;border-left:3px solid var(--b2);border-radius:14px;padding:14px;display:grid;gap:10px;transition:.2s}
.card:hover{border-color:#e85d044d;box-shadow:0 0 24px #e85d0414;transform:translateY(-1px)}
.card[data-s=New]{border-left-color:#7c7870}.card[data-s=Called]{border-left-color:var(--a2)}.card[data-s=Messaged]{border-left-color:#60a5fa}.card[data-s=Replied]{border-left-color:#c084fc}.card[data-s=Pilot]{border-left-color:#22d3ee}.card[data-s=Paid]{border-left-color:var(--g)}.card[data-s=Dead]{border-left-color:var(--r);opacity:.55}
.row{display:flex;gap:7px;align-items:center;flex-wrap:wrap}.row b{font:700 16px var(--head);letter-spacing:-.01em}.m{color:var(--m);font-size:12px}
.tag{background:var(--c2);border:1px solid var(--b);border-radius:99px;padding:1px 9px;font-size:12px;color:var(--m)}
.q{background:#e342340f;border-left:3px solid var(--r);padding:7px 10px;font-size:12px;border-radius:6px;color:#e8c9c5}
.call{display:grid;grid-template-columns:auto 1fr 1fr;gap:6px;align-items:center;background:var(--bg);border:1px solid var(--b);padding:8px;border-radius:10px}.call input,.call select{padding:4px 6px;font-size:12px;min-width:0}
.cts{display:grid;gap:5px}.ct{display:flex;align-items:center;gap:6px;background:var(--bg);border:1px solid var(--b);border-radius:9px;padding:4px 8px}.ct .k{width:62px;font-size:10.5px;text-transform:uppercase;letter-spacing:.6px;color:var(--mm)}.ct .v{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font:500 12.5px var(--mono)}
a.s{display:inline-block;padding:2px 9px;font-size:12px;border:1px solid var(--b);border-radius:7px;color:var(--t);text-decoration:none;background:var(--c2)}a.s:hover{border-color:var(--a)}
.bd{display:inline-block;border-radius:99px;padding:1px 9px;font-size:11.5px;font-weight:600}.bd.A{background:#4ade8022;color:#4ade80}.bd.B{background:#60a5fa22;color:#7db4ff}.bd.hot{background:#e8420422;color:#ff7a3d}.bd.warn{background:#faa30722;color:var(--a2)}.bd.due{background:#c084fc22;color:#d3a4ff}.bd.go{background:#22d3ee1f;color:#5fe3f5}
.card textarea{min-height:44px;font-size:12px;padding:5px 8px;resize:vertical}
.ch{display:flex;gap:6px;flex-wrap:wrap}.ch button{border:0;color:#fff;padding:7px 13px;border-radius:99px;font-size:12.5px;font-weight:600}
.ch [data-ch=sms]{background:#0284c7}.ch [data-ch=whatsapp]{background:#1fa855}.ch [data-ch=email]{background:#5b5bd6}.ch [data-ch=facebook]{background:#1877f2}.ch [data-ch=instagram]{background:linear-gradient(45deg,#f58529,#dd2a7b,#8134af)}.ch button:hover{filter:brightness(1.12);transform:translateY(-1px)}
dialog{border:1px solid #ffffff14;border-radius:16px;background:#141413f2;backdrop-filter:blur(14px);color:var(--t);width:min(560px,94vw);max-height:92vh;overflow:auto;padding:18px;box-shadow:0 24px 70px #000a,0 0 40px #e85d040f}dialog::backdrop{background:#000a;backdrop-filter:blur(3px)}
textarea{width:100%;min-height:150px;resize:vertical;line-height:1.55}.tplb{margin:10px 0}.tplb textarea{min-height:120px;font:12.5px/1.5 var(--mono)}.err{color:var(--r)}.ok{color:var(--g)}
#login{position:fixed;inset:0;background:radial-gradient(700px 420px at 50% 30%,#e85d0422,transparent 65%),var(--bg);display:none;place-items:center;z-index:9}
#login div{background:#141413cc;border:1px solid #ffffff14;border-radius:16px;padding:28px;display:grid;gap:12px;width:min(350px,92vw);box-shadow:0 0 60px #e85d040f}#login b{font:800 22px var(--head);background:linear-gradient(135deg,var(--a),var(--a2));-webkit-background-clip:text;background-clip:text;-webkit-text-fill-color:transparent}
#toast{position:fixed;bottom:24px;left:50%;transform:translateX(-50%) translateY(20px);background:var(--c2);border:1px solid var(--a);color:var(--t);padding:8px 18px;border-radius:99px;opacity:0;pointer-events:none;transition:.25s;z-index:20;font-size:13px;box-shadow:0 0 24px #e85d0433}#toast.on{opacity:1;transform:translateX(-50%)}
::-webkit-scrollbar{width:10px;height:10px}::-webkit-scrollbar-thumb{background:var(--b2);border-radius:99px}
@media(prefers-reduced-motion:reduce){button.p{animation:none}.card{transition:none}}
@media(max-width:480px){#list{grid-template-columns:1fr;padding:6px 12px 60px}header{padding:10px 12px}}
.tabs{display:flex;gap:4px;background:var(--c2);border:1px solid var(--b);border-radius:99px;padding:3px}.tab{border:0;background:none;border-radius:99px;padding:5px 15px;color:var(--m);font-weight:600}.tab.on{background:linear-gradient(135deg,var(--a),var(--a2));color:#fff}
.calls{display:grid;gap:6px;background:var(--bg);border:1px solid var(--b);border-radius:10px;padding:8px}.cr{display:flex;gap:6px;align-items:center;flex-wrap:wrap}.cr .m:first-child{width:44px}
.cb{border-radius:99px!important;padding:3px 11px!important}.cb.Missed:hover{border-color:var(--r);color:#ff8a80}.cb.Answered:hover{border-color:var(--g);color:var(--g)}.cb.Voicemail:hover{border-color:var(--a2);color:var(--a2)}
.res{font-weight:700;font-size:12.5px;padding:1px 10px;border-radius:99px}.res.Missed{background:#e3423422;color:#ff8a80}.res.Answered{background:#4ade8022;color:var(--g)}.res.Voicemail{background:#faa30722;color:var(--a2)}
#toast.act{pointer-events:auto}#toast .s{margin-left:8px;border-color:var(--a);color:var(--a2)}
#rv{max-width:1100px;margin:auto;padding:14px 18px 70px;display:grid;gap:16px}.kpis{display:grid;grid-template-columns:repeat(auto-fill,minmax(140px,1fr));gap:10px}
.kpi{background:#141413b3;border:1px solid #ffffff12;border-radius:12px;padding:12px}.kpi b{display:block;font:800 26px var(--head);letter-spacing:-.02em}.kpi span{font-size:11.5px;color:var(--m);text-transform:uppercase;letter-spacing:.5px}.kpi.hl b{background:linear-gradient(135deg,var(--a),var(--a2));-webkit-background-clip:text;background-clip:text;-webkit-text-fill-color:transparent}
.rbox{background:#141413b3;border:1px solid #ffffff12;border-radius:14px;padding:14px;overflow-x:auto}.rbox h3{margin:0 0 10px;font:700 15px var(--head)}
table{border-collapse:collapse;width:100%;font-size:13px}th,td{text-align:left;padding:6px 10px;border-bottom:1px solid var(--b)}th{color:var(--mm);font-size:11px;text-transform:uppercase;letter-spacing:.5px;font-weight:600}td.n{font-family:var(--mono)}tr.today td{color:var(--a2)}
#rt{font:12.5px/1.55 var(--mono);min-height:240px}
#list{grid-template-columns:repeat(auto-fill,minmax(min(350px,100%),1fr))}
.card,.cts,.calls{grid-template-columns:minmax(0,1fr)}.card{min-width:0;overflow-wrap:anywhere}.card>*{min-width:0}
.card textarea,.card input{max-width:100%}.card .row b{overflow-wrap:anywhere}
.ct{min-width:0}.ct .k{flex:none}.ct button,.ct a.s{flex:none}.ct .v{overflow-wrap:normal}
.q{overflow-wrap:anywhere}.card select[data-f=status]{max-width:100%}
</style></head><body>
<div id="login"><div><b>Leadomi</b><span class="m">Outreach console</span><input id="pw" type="password" placeholder="Passcode"><button class="p" id="lg">Log in</button><span id="le" class="err"></span></div></div>
<header><h1><span class="gt">Leadomi</span><span class="dim">Outreach</span></h1><span class="tabs"><button class="tab on" id="tl">Leads</button><button class="tab" id="tr">Results</button></span><span class="pill" id="cnt">Sent today 0/20</span><button id="imp">Import</button><a href="/api/export"><button>Export</button></a><button id="st">Settings</button><input type="file" id="file" accept=".csv" hidden></header>
<div id="pbw"><div id="pb"></div></div><div class="bar"><input id="q" placeholder="Search business, city, phone, owner"><select id="ft"><option value="">All tiers</option><option value="A">Tier A</option><option value="B">Tier B</option></select><select id="fc"><option value="">All cities</option></select><select id="so"><option value="pri">Sort: Priority</option><option value="due">Sort: Follow-up due</option><option value="rev">Sort: Newest review</option><option value="upd">Sort: Recently updated</option><option value="name">Sort: Name A-Z</option></select></div><div class="chips" id="chips"></div><div class="cnt m" id="shown"></div><div id="toast"></div>
<div id="list"></div>
<div id="rv" hidden>
<div class="row"><b class="hd">Results</b><input type="date" id="rd"><span class="m">Calls are counted by their UK date, messages by your local date.</span></div>
<div class="kpis" id="kp"></div>
<div class="rbox"><h3>Last 7 days</h3><div id="r7"></div></div>
<div class="rbox"><h3>Funnel (all time)</h3><div id="rf"></div></div>
<div class="rbox"><h3>By city</h3><div id="rc"></div></div>
<div class="rbox"><h3>Report to send me</h3><label class="m">Anything stuck or confusing today? (optional)<br><input id="rn" style="width:100%;margin:4px 0 10px"></label><textarea id="rt" readonly></textarea><div class="row" style="margin-top:10px"><button class="p" id="rcp">Copy report</button></div></div>
</div>
<dialog id="md"><div class="row"><b id="mt" style="flex:1"></b><select id="mty"><option value="first">First message</option><option value="follow1">Follow-up 1</option><option value="follow2">Follow-up 2</option></select></div>
<div id="mld" class="m" style="padding:30px 0;text-align:center">Preparing message...</div>
<div id="mbd" hidden><input id="msub" placeholder="Subject" style="width:100%;margin:8px 0" hidden><textarea id="mtx"></textarea><div class="m" id="mpv"></div></div>
<div class="row" style="margin-top:10px"><button id="mx">Close</button><span style="flex:1"></span><button id="mcp" disabled>Copy</button><button id="mre" disabled>Next variant</button><button class="p" id="msd" disabled>Send</button></div></dialog>
<dialog id="sd"><b class="hd">Settings</b>
<div class="row" style="margin:12px 0"><label class="m">Demo number<br><input id="sdemo" placeholder="07..." style="width:170px"></label><label class="m">Daily quota<br><input id="sq" type="number" min="1" style="width:90px"></label></div>
<div class="m" style="margin-bottom:8px">Variables: {name} {garage} {day} {time} {town} {demo} {pain}. Separate variants with a line containing only ---. Variants rotate automatically so no two messages in a row match.</div>
<div id="sb"></div>
<div class="row" style="margin-top:12px"><button id="sx">Close</button><button id="sr">Reset templates</button><span style="flex:1"></span><button class="p" id="ss">Save</button></div></dialog>
<script>
const $=s=>document.querySelector(s);let leads=[],cfg,cur={};
const esc=s=>String(s==null?"":s).replace(/[&<>"]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]));
async function api(p,o){const mt=o?(o.m||"POST"):"GET";const r=await fetch(p,mt==="GET"?{}:{method:mt,headers:{"content-type":"application/json"},body:JSON.stringify((o&&o.b)||{})});if(r.status===401&&p!=="/api/login"){$("#login").style.display="grid";throw new Error("auth")}
 const j=await r.json().catch(()=>({}));if(!r.ok)throw new Error(j.error||("Error "+r.status));return j}
const phoneIntl=p=>{let d=String(p||"").replace(/[^\d+]/g,"");if(d.startsWith("+"))return d;if(d.startsWith("00"))return "+"+d.slice(2);if(d.startsWith("0"))return "+44"+d.slice(1);return "+"+d};
// Calls and texts go through the Leadomi SIP server (business number +447782226484).
// PC: opens the SIP console Dialer / Messages prefilled. Phone: Call opens Linphone.
const SIP="https://sip.leadomi.com",SIP_HOST="sip.leadomi.com";
const MOB=/Android|iPhone|iPad|iPod/i.test(navigator.userAgent);
const callUrl=p=>MOB?"sip:"+phoneIntl(p)+"@"+SIP_HOST:SIP+"/dialer?to="+encodeURIComponent(phoneIntl(p));
const textUrl=(p,body)=>SIP+"/messages?to="+encodeURIComponent(phoneIntl(p))+"&body="+encodeURIComponent(body||"");
function copy(t){return navigator.clipboard.writeText(t).catch(()=>{const a=document.createElement("textarea");a.value=t;document.body.appendChild(a);a.select();document.execCommand("copy");a.remove()})}
function flash(b,t){const o=b.textContent;b.textContent="Copied";toast(t||"Copied to clipboard");setTimeout(()=>b.textContent=o,900)}
async function login(){try{await api("/api/login",{b:{passcode:$("#pw").value}});$("#login").style.display="none";$("#pw").value="";boot()}catch(e){$("#le").textContent=e.message}}
$("#lg").onclick=login;$("#pw").onkeydown=e=>{if(e.key==="Enter")login()};
async function boot(){leads=await api("/api/leads");cities();render();stats()}
async function stats(){const s=await api("/api/stats?tz="+(-new Date().getTimezoneOffset()));$("#cnt").textContent="Sent today "+s.sent+"/"+s.quota;$("#pb").style.width=Math.min(100,s.sent/s.quota*100)+"%"}
const ST=["New","Called","Messaged","Replied","Pilot","Paid","Dead"],RS=["","Answered","Missed","Voicemail"];
const opts=(a,v)=>a.map(x=>"<option"+(x===v?" selected":"")+">"+x+"</option>").join("");
let FS="";
const short=v=>String(v).replace(/^https?:\/\/(www\.)?/,"").replace(/\/$/,"").slice(0,36);
const href=(k,v)=>k==="Phone"?callUrl(v):k==="Email"?"mailto:"+v:(/^https?:/i.test(v)?v:"https://"+v);
const dago=s=>s?Math.floor((Date.now()-new Date(s))/864e5):null;
const strong=l=>l.call1_result&&l.call1_result!=="Answered"&&l.call2_result&&l.call2_result!=="Answered";
const tier=l=>{const m=/Tier (A|B)/.exec(l.notes||"");return m?m[1]:/Inactive/.test(l.notes||"")?"X":""};
const due=l=>l.status==="Messaged"&&l.last_sent&&dago(l.last_sent)>=2?1:0;
const rank=l=>({A:0,B:1,"":2,X:3})[tier(l)]-(l.pain_quote?.5:0)+(l.call1_result==="Answered"?5:0);
const SORTS={pri:(a,b)=>rank(a)-rank(b)||(b.last_review_date||"").localeCompare(a.last_review_date||""),due:(a,b)=>due(b)-due(a)||SORTS.pri(a,b),name:(a,b)=>a.business.localeCompare(b.business),rev:(a,b)=>(b.last_review_date||"").localeCompare(a.last_review_date||""),upd:(a,b)=>(b.updated_at||"").localeCompare(a.updated_at||"")};
let tt;function toast(t){const e=$("#toast");e.textContent=t;e.classList.remove("act");e.classList.add("on");clearTimeout(tt);tt=setTimeout(()=>e.classList.remove("on"),1400)}
function cities(){const sel=$("#fc"),v=sel.value,cs=[...new Set(leads.map(l=>l.city).filter(Boolean))].sort();sel.innerHTML='<option value="">All cities</option>'+cs.map(c=>'<option'+(c===v?" selected":"")+'>'+esc(c)+'</option>').join("")}
function render(){const q=$("#q").value.toLowerCase(),tr=$("#ft").value,ci=$("#fc").value;
 const cnt={};leads.forEach(l=>cnt[l.status]=(cnt[l.status]||0)+1);
 $("#chips").innerHTML=['<button data-s="" class="'+(FS?"":"on")+'">All '+leads.length+'</button>'].concat(ST.map(x=>'<button data-s="'+x+'" class="'+(FS===x?"on":"")+'">'+x+' '+(cnt[x]||0)+'</button>')).join("");
 const L=leads.filter(l=>(!FS||l.status===FS)&&(!tr||tier(l)===tr)&&(!ci||l.city===ci)&&(!q||[l.business,l.city,l.phone,l.owner_name].join(" ").toLowerCase().includes(q))).sort(SORTS[$("#so").value]);
 $("#shown").textContent="Showing "+L.length+" of "+leads.length+" leads";
 $("#list").innerHTML=L.map(l=>{
 const ch=[];if(l.phone){ch.push(["sms","Send a Text Message"],["whatsapp","Message on WhatsApp"])}if(l.email)ch.push(["email","Send an Email"]);if(l.facebook)ch.push(["facebook","Message on Facebook"]);if(l.instagram)ch.push(["instagram","Message on Instagram"]);
 const t=tier(l),rd=dago(l.last_review_date),bd=[];
 if(t==="A"||t==="B")bd.push('<span class="bd '+t+'">Tier '+t+'</span>');if(l.pain_quote)bd.push('<span class="bd hot">Hot lead</span>');
 if(l.call1_result==="Answered")bd.push('<span class="bd warn">Answered - skip</span>');if(strong(l))bd.push('<span class="bd go">Missed both calls</span>');if(due(l))bd.push('<span class="bd due">Follow-up due</span>');
 if(/Reputation risk/.test(l.notes||""))bd.push('<span class="bd warn">Reputation risk</span>');if(/replies to reviews/.test(l.notes||""))bd.push('<span class="bd go">Replies to reviews</span>');
 if(rd!==null)bd.push('<span class="tag">Last review '+(rd<=0?"today":rd+"d ago")+'</span>');if(l.sent_count)bd.push('<span class="tag">'+l.sent_count+' sent</span>');
 const C=[["Phone",l.phone],["Email",l.email],["Facebook",l.facebook],["Instagram",l.instagram],["Website",l.website]].filter(x=>x[1]);
 return '<div class="card" data-id="'+l.id+'" data-s="'+esc(l.status)+'"><div class="row"><b>'+esc(l.business)+'</b><button class="s" data-c="'+esc(l.business)+'">Copy</button><select data-f="status" style="margin-left:auto">'+opts(ST,l.status)+'</select></div>'+
 '<div class="row"><span class="tag">'+esc(l.city)+'</span>'+(l.rating?'<span class="tag">'+l.rating+' stars - '+l.reviews+' reviews</span>':'')+bd.join("")+'</div>'+
 '<div class="cts">'+C.map(x=>'<div class="ct"><span class="k">'+x[0]+'</span><span class="v" title="'+esc(x[1])+'">'+esc(x[0]==="Phone"||x[0]==="Email"?x[1]:short(x[1]))+'</span><button class="s" data-c="'+esc(x[1])+'">Copy</button><a class="s" target="_blank" rel="noopener" href="'+esc(href(x[0],x[1]))+'">'+(x[0]==="Phone"?"Call":"Open")+'</a></div>').join("")+'</div>'+
 '<div class="row"><span class="m">Owner</span><input data-f="owner_name" value="'+esc(l.owner_name)+'" placeholder="first name" style="width:130px;padding:4px 8px"></div>'+
 (l.pain_quote?'<div class="q">'+esc(l.pain_quote)+'</div>':'')+
 '<div class="calls">'+callRow(l,1)+callRow(l,2)+'</div>'+
 (["Replied","Pilot","Paid"].includes(l.status)?'<textarea data-f="reply_text" placeholder="Paste their reply here (so it shows in your report)">'+esc(l.reply_text)+'</textarea>':'')+
 '<textarea data-f="notes" placeholder="Notes">'+esc(l.notes)+'</textarea>'+
 '<div class="ch">'+ch.map(c=>'<button data-ch="'+c[0]+'">'+c[1]+'</button>').join("")+'</div></div>'}).join("")||'<div class="m" style="padding:30px">No leads match. Change the filters or click Import.</div>'}
$("#chips").onclick=e=>{const b=e.target.closest("button");if(!b)return;FS=b.dataset.s;render()};
["#q","#ft","#fc","#so"].forEach(x=>$(x).oninput=render);
function ukNow(){const p=new Intl.DateTimeFormat("en-GB",{timeZone:"Europe/London",year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",hourCycle:"h23"}).formatToParts(new Date());const g=t=>p.find(x=>x.type===t).value;return g("year")+"-"+g("month")+"-"+g("day")+"T"+g("hour")+":"+g("minute")}
function fmtUk(s){if(!s)return"";const d=new Date(s+":00Z");if(isNaN(d))return s;return d.toLocaleDateString("en-GB",{weekday:"short",timeZone:"UTC"})+" "+String(d.getUTCHours()).padStart(2,"0")+":"+String(d.getUTCMinutes()).padStart(2,"0")}
function callRow(l,n){const at=l["call"+n+"_at"],rs=l["call"+n+"_result"];
 if(rs)return '<div class="cr"><span class="m">Call '+n+'</span><span class="res '+rs+'">'+rs+'</span><span class="m">'+fmtUk(at)+' UK</span>'+(n===1&&l.call2_result?'':'<button class="s" data-undo="'+n+'">Undo</button>')+'</div>';
 const dis=n===2&&!l.call1_result?" disabled":"";
 return '<div class="cr"><span class="m">Call '+n+'</span>'+["Missed","Voicemail","Answered"].map(x=>'<button class="s cb '+x+'" data-call="'+n+':'+x+'"'+dis+'>'+x+'</button>').join("")+'</div>'}
function toastUndo(t,fn){const e=$("#toast");e.innerHTML=esc(t)+'<button class="s" id="tu">Undo</button>';e.classList.add("on","act");clearTimeout(tt);$("#tu").onclick=()=>{e.classList.remove("on","act");fn()};tt=setTimeout(()=>e.classList.remove("on","act"),9000)}
async function patch(id,b){await api("/api/leads/"+id,{m:"PATCH",b})}
async function logCall(id,n,res){const l=leads.find(x=>x.id===id),at=ukNow(),ps=l.status;l["call"+n+"_at"]=at;l["call"+n+"_result"]=res;if(res!=="Answered"&&l.status==="New")l.status="Called";
 render();await patch(id,{["call"+n+"_at"]:at,["call"+n+"_result"]:res,status:l.status});
 toastUndo("Call "+n+" "+res.toLowerCase()+" logged at "+fmtUk(at)+" UK",()=>undoCall(id,n,ps))}
async function undoCall(id,n,ps){const l=leads.find(x=>x.id===id);l["call"+n+"_at"]=null;l["call"+n+"_result"]=null;
 if(ps!==undefined)l.status=ps;else if(l.status==="Called"&&!l.call1_result&&!l.call2_result)l.status="New";
 render();await patch(id,{["call"+n+"_at"]:"",["call"+n+"_result"]:"",status:l.status});toast("Undone")}
$("#list").onclick=e=>{const b=e.target.closest("button");if(!b)return;if(b.dataset.c!==undefined){copy(b.dataset.c);flash(b);return}
 const id=+b.closest(".card")?.dataset.id;if(b.dataset.call){const[n,r]=b.dataset.call.split(":");logCall(id,+n,r);return}
 if(b.dataset.undo){undoCall(id,+b.dataset.undo);return}
 if(b.dataset.ch)openMsg(id,b.dataset.ch)};
$("#list").onchange=async e=>{const f=e.target.dataset.f;if(!f)return;const id=+e.target.closest(".card").dataset.id,l=leads.find(x=>x.id===id),prev=l[f];l[f]=e.target.value;
 await patch(id,{[f]:l[f]});if(f==="status"){render();toastUndo("Status: "+prev+" to "+l.status,async()=>{l.status=prev;await patch(id,{status:prev});render()})}};
function link(ch,l,body,sub){const t=encodeURIComponent(body),p=phoneIntl(l.phone);
 if(ch==="whatsapp")return "https://wa.me/"+p.replace("+","")+"?text="+t;if(ch==="sms")return textUrl(p,body);
 if(ch==="email")return "mailto:"+l.email+"?subject="+encodeURIComponent(sub||"")+"&body="+t;
 if(ch==="facebook"){const m=String(l.facebook).match(/facebook\.com\/([^/?#]+)/i);return m&&!/^(profile\.php|pages|people|p)$/i.test(m[1])?"https://m.me/"+m[1]:l.facebook}
 const m=String(l.instagram).match(/instagram\.com\/([^/?#]+)/i);return m?"https://ig.me/m/"+m[1]:l.instagram}
const CN={sms:"Text Message",whatsapp:"WhatsApp",email:"Email",facebook:"Facebook",instagram:"Instagram"};
function openMsg(id,ch){cur={id,ch,l:leads.find(x=>x.id===id),mid:0};$("#mt").textContent=cur.l.business+" - "+CN[ch];$("#mty").value=["first","follow1","follow2"][Math.min(cur.l.sent_count||0,2)];$("#md").showModal();gen()}
async function gen(){["#mcp","#mre","#msd"].forEach(s=>$(s).disabled=true);$("#mbd").hidden=true;$("#mld").hidden=false;$("#mld").className="m";$("#mld").textContent="Preparing message...";
 try{const r=await api("/api/generate",{b:{lead_id:cur.id,channel:cur.ch,type:$("#mty").value}});let b=r.body,s="";
  if(cur.ch==="email"){const m=b.match(/^SUBJECT:\s*(.*)\n+([\s\S]*)$/i);if(m){s=m[1];b=m[2]}else s="Quick question"}
  cur.mid=r.id;$("#msub").hidden=cur.ch!=="email";$("#msub").value=s;$("#mtx").value=b;$("#mpv").textContent=r.provider+": "+r.model;
  $("#mld").hidden=true;$("#mbd").hidden=false;["#mcp","#mre","#msd"].forEach(x=>$(x).disabled=false)}
 catch(e){$("#mld").className="err";$("#mld").textContent=e.message;$("#mre").disabled=false}}
$("#mty").onchange=gen;$("#mre").onclick=gen;$("#mx").onclick=()=>$("#md").close();
$("#mcp").onclick=e=>{copy($("#mtx").value);flash(e.target,"Message copied")};
$("#msd").onclick=async()=>{const body=$("#mtx").value,sub=$("#msub").value;await copy(body);window.open(link(cur.ch,cur.l,body,sub),"_blank");
 try{const ps=cur.l.status,mid=cur.mid,L=cur.l;await api("/api/messages/"+mid+"/sent",{b:{body}});if(["New","Called"].includes(L.status))L.status="Messaged";L.sent_count=(L.sent_count||0)+1;L.last_sent=new Date().toISOString();render();stats();
  toastUndo("Marked as sent",async()=>{await api("/api/messages/"+mid+"/unsent",{b:{}});if(L.status!==ps){L.status=ps;await patch(L.id,{status:ps})}boot()})}catch(e){}
 $("#mpv").textContent=cur.ch==="sms"?"Opened in Leadomi SIP Messages with the text ready. Press Send there.":"Copied and opened. Paste it if the chat did not prefill."};
$("#imp").onclick=()=>$("#file").click();
function parseCSV(t){const R=[];let r=[],c="",q=false;for(let i=0;i<t.length;i++){const ch=t[i];if(q){if(ch==='"'){if(t[i+1]==='"'){c+='"';i++}else q=false}else c+=ch}else if(ch==='"')q=true;else if(ch===","){r.push(c);c=""}else if(ch==="\n"||ch==="\r"){if(ch==="\r"&&t[i+1]==="\n")i++;r.push(c);c="";if(r.some(x=>x))R.push(r);r=[]}else c+=ch}if(c||r.length){r.push(c);R.push(r)}return R}
const AL={business:["business","title","name"],city:["city"],phone:["phone"],website:["website"],email:["email"],instagram:["instagram"],facebook:["facebook"],rating:["rating","totalscore"],reviews:["reviews","reviewscount"],owner_name:["owner_name","owner"],last_review_date:["last_review_date"],pain_quote:["pain_quote"],status:["status"],notes:["notes"],call1_at:["call1_at"],call1_result:["call1_result"],call2_at:["call2_at"],call2_result:["call2_result"]};
$("#file").onchange=async e=>{const f=e.target.files[0];if(!f)return;const R=parseCSV(await f.text()),H=R[0].map(h=>h.trim().toLowerCase().replace(/[^a-z0-9_]/g,""));
 const rows=R.slice(1).map(r=>{const o={};for(const k in AL){const i=H.findIndex(h=>AL[k].includes(h));if(i>=0)o[k]=(r[i]||"").trim()}if(o.phone)o.phone=phoneIntl(o.phone);return o});
 let a=0,s=0;for(let i=0;i<rows.length;i+=100){const r=await api("/api/leads/import",{b:{rows:rows.slice(i,i+100)}});a+=r.added;s+=r.skipped}
 alert("Imported or updated "+a+" leads. Skipped "+s+" (missing name or phone).");e.target.value="";boot()};
const TL={first:"First message (uses the call day and time)",nocall:"First message (when no call is logged)",follow1:"Follow-up 1",follow2:"Follow-up 2"};
async function openSet(){const r=await api("/api/settings",{m:"GET"});cfg=r.cfg;drawSet();$("#sd").showModal()}
function drawSet(){$("#sdemo").value=cfg.demo||"";$("#sq").value=cfg.quota;$("#sb").innerHTML=Object.keys(TL).map(k=>'<div class="tplb"><div class="m">'+TL[k]+'</div><textarea data-k="'+k+'">'+esc(cfg.tpl[k].join("\n---\n"))+'</textarea></div>').join("")}
function readSet(){cfg.demo=$("#sdemo").value.trim();cfg.quota=+$("#sq").value||20;document.querySelectorAll("#sb textarea").forEach(t=>{const a=t.value.split(/\n\s*---\s*\n/).map(x=>x.trim()).filter(Boolean);if(a.length)cfg.tpl[t.dataset.k]=a})}
$("#ss").onclick=async()=>{readSet();await api("/api/settings",{b:cfg});$("#sd").close();stats();toast("Settings saved")};
$("#sr").onclick=async()=>{if(!confirm("Reset all templates to the defaults?"))return;const r=await api("/api/settings",{b:{reset:1}});cfg=r.cfg;drawSet();toast("Templates reset")};
$("#sx").onclick=()=>$("#sd").close();$("#st").onclick=openSet;
let SENT=[];const dloc=d=>new Date(d).toLocaleDateString("en-CA");
function tab(t){const r=t==="r";$("#tl").classList.toggle("on",!r);$("#tr").classList.toggle("on",r);[".bar","#chips","#shown","#list"].forEach(x=>$(x).hidden=r);$("#rv").hidden=!r;if(r){if(!$("#rd").value)$("#rd").value=dloc(new Date());loadRes()}}
$("#tl").onclick=()=>tab("l");$("#tr").onclick=()=>tab("r");
async function loadRes(){leads=await api("/api/leads");SENT=await api("/api/messages/sent");renderRes()}
function dayStats(day){const S={calls:0,missed:0,vm:0,ans:0,strong:0,msgs:0,rep:0,pil:0,paid:0,ch:{}};
 for(const l of leads){for(const n of [1,2]){const r=l["call"+n+"_result"],at=l["call"+n+"_at"];if(r&&(at||"").slice(0,10)===day){S.calls++;if(r==="Missed")S.missed++;else if(r==="Voicemail")S.vm++;else S.ans++}}
  if(strong(l)&&(l.call2_at||"").slice(0,10)===day)S.strong++;
  if(l.replied_at&&dloc(l.replied_at)===day)S.rep++;if(l.pilot_at&&dloc(l.pilot_at)===day)S.pil++;if(l.paid_at&&dloc(l.paid_at)===day)S.paid++}
 for(const m of SENT)if(dloc(m.sent_at)===day){S.msgs++;S.ch[m.channel]=(S.ch[m.channel]||0)+1}
 S.mr=S.calls?Math.round((S.missed+S.vm)/S.calls*100):0;S.chs=Object.keys(S.ch).map(k=>CN[k]+" "+S.ch[k]).join(", ");return S}
const tbl=(h,rows)=>'<table><tr>'+h.map(x=>'<th>'+x+'</th>').join("")+'</tr>'+rows.join("")+'</table>';
function totals(){const T={leads:leads.length,called:0,msg:0,rep:0,pil:0,paid:0};for(const l of leads){if(l.call1_result)T.called++;if(l.sent_count>0)T.msg++;if(l.replied_at||["Replied","Pilot","Paid"].includes(l.status))T.rep++;if(l.pilot_at||["Pilot","Paid"].includes(l.status))T.pil++;if(l.paid_at||l.status==="Paid")T.paid++}return T}
function cityRows(){const C={};for(const l of leads){const c=l.city||"-",o=C[c]||(C[c]={n:0,call:0,miss:0,msg:0,rep:0});o.n++;if(l.call1_result){o.call++;if(l.call1_result!=="Answered")o.miss++}if(l.sent_count>0)o.msg++;if(l.replied_at||["Replied","Pilot","Paid"].includes(l.status))o.rep++}return Object.entries(C).sort((a,b)=>b[1].n-a[1].n)}
function renderRes(){const day=$("#rd").value||dloc(new Date()),S=dayStats(day),T=totals();
 const K=[["Calls made",S.calls,1],["Missed",S.missed],["Voicemail",S.vm],["Answered",S.ans],["Miss rate",S.mr+"%"],["Strong leads",S.strong,1],["Messages sent",S.msgs,1],["Replies",S.rep,1],["Pilots",S.pil,1],["Paid",S.paid,1]];
 $("#kp").innerHTML=K.map(k=>'<div class="kpi'+(k[2]?" hl":"")+'"><b>'+k[1]+'</b><span>'+k[0]+'</span></div>').join("");
 const rows=[];for(let i=0;i<7;i++){const d=dloc(new Date(Date.now()-i*864e5)),x=dayStats(d);rows.push('<tr'+(d===day?' class="today"':'')+'><td>'+d+'</td><td class="n">'+x.calls+'</td><td class="n">'+(x.missed+x.vm)+'</td><td class="n">'+x.ans+'</td><td class="n">'+x.msgs+'</td><td class="n">'+x.rep+'</td><td class="n">'+x.pil+'</td><td class="n">'+x.paid+'</td></tr>')}
 $("#r7").innerHTML=tbl(["Date","Calls","Missed","Answered","Sent","Replies","Pilots","Paid"],rows);
 const pc=(a,b)=>b?Math.round(a/b*100)+"%":"-";
 $("#rf").innerHTML=tbl(["Stage","Leads","Of previous"],[["Leads",T.leads,""],["Called",T.called,pc(T.called,T.leads)],["Messaged",T.msg,pc(T.msg,T.called)],["Replied",T.rep,pc(T.rep,T.msg)],["Pilot",T.pil,pc(T.pil,T.rep)],["Paid",T.paid,pc(T.paid,T.pil)]].map(r=>'<tr><td>'+r[0]+'</td><td class="n">'+r[1]+'</td><td class="n">'+r[2]+'</td></tr>'));
 $("#rc").innerHTML=tbl(["City","Leads","Called","Missed","Messaged","Replied"],cityRows().map(([c,o])=>'<tr><td>'+esc(c)+'</td><td class="n">'+o.n+'</td><td class="n">'+o.call+'</td><td class="n">'+o.miss+'</td><td class="n">'+o.msg+'</td><td class="n">'+o.rep+'</td></tr>'));
 const R=leads.filter(l=>l.replied_at&&dloc(l.replied_at)===day);
 const L=["Leadomi report - "+day,"Calls made: "+S.calls+" | Missed: "+S.missed+" | Voicemail: "+S.vm+" | Answered: "+S.ans+" | Miss rate: "+S.mr+"%","Strong leads (missed both calls): "+S.strong,"Messages sent: "+S.msgs+(S.chs?" ("+S.chs+")":""),"Replies: "+S.rep+" | Pilots: "+S.pil+" | Paid: "+S.paid,"","Totals: leads "+T.leads+", called "+T.called+", messaged "+T.msg+", replied "+T.rep+", pilots "+T.pil+", paid "+T.paid,"","By city:"].concat(cityRows().map(([c,o])=>"- "+c+": "+o.n+" leads, "+o.call+" called, "+o.miss+" missed, "+o.msg+" messaged, "+o.rep+" replied"));
 if(R.length){L.push("","Replies today:");R.forEach(l=>L.push("- "+l.business+" ("+(l.city||"")+") ["+l.status+"]: "+(l.reply_text||"(no reply text saved)")))}
 const n=$("#rn").value.trim();if(n)L.push("","Stuck / notes: "+n);$("#rt").value=L.join("\n")}
$("#rd").onchange=renderRes;$("#rn").oninput=renderRes;$("#rcp").onclick=e=>{copy($("#rt").value);flash(e.target,"Report copied")};
api("/api/me",{m:"GET"}).then(boot).catch(()=>{});
</script></body></html>`;
