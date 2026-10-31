"""Asterisk lifecycle, authenticated FastAGI SMS, and persistent inbound delivery.

The Leadomi SIP database is read-only here. Twilio API credentials never enter this
container. Each endpoint has its own message context: sender identity cannot
be forged by changing a SIP From header or an arbitrary MESSAGE header.
"""
import base64
import hashlib
import hmac
import http.server
import ipaddress
import json
import logging
import os
from pathlib import Path
import re
import secrets
import signal
import socket
import socketserver
import sqlite3
import subprocess
import threading
import time
import urllib.error
import urllib.request

LOG = logging.getLogger("gosip-pbx")
USERNAME = re.compile(r"^[A-Za-z0-9_-]{1,64}$")
# Asterisk passwords are values, not section/endpoint names. Permit common
# punctuation while rejecting comments, quotes, escapes and line injection.
PASSWORD = re.compile(r"^[A-Za-z0-9_@#!$%&*()+.,=:-]{1,128}$")
NUMBER = re.compile(r"^\+[1-9][0-9]{7,14}$")
DB = os.getenv("GOSIP_DB", "/gosip/gosip.db")
STATE = Path(os.getenv("PBX_STATE_DIR", "/var/lib/gosip-pbx"))
CONF = Path(os.getenv("PBX_CONFIG_DIR", "/etc/asterisk"))
DOMAIN = os.getenv("PBX_DOMAIN", "sip.example.com")
SECRET = os.getenv("GOSIP_PBX_SECRET", "")
GOSIP = os.getenv("GOSIP_URL", "http://gosip:8080").rstrip("/")
AMI_SECRET = secrets.token_hex(32)
PROCESS = None
STOP = threading.Event()
LAST_POLL = 0
CONFIG_LOCK = threading.RLock()


def safe(value, pattern=USERNAME):
    if not pattern.fullmatch(value):
        raise ValueError("Invalid PBX configuration value")
    return value


def source():
    conn = sqlite3.connect("file:" + DB + "?mode=ro", uri=True, timeout=5)
    conn.row_factory = sqlite3.Row
    return conn


def atomic(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".new")
    temporary.write_text(content, encoding="utf-8")
    temporary.chmod(0o600)
    temporary.replace(path)


def cli(command):
    output = subprocess.run(["asterisk", "-rx", command], capture_output=True,
                            text=True, timeout=15, check=True).stdout
    if "No such command" in output or "No such module" in output:
        raise RuntimeError("Asterisk command is unavailable")
    return output


def reload_sip():
    cli("module reload res_pjsip.so")
    cli("module reload res_pjsip_endpoint_identifier_ip.so")


def render(devices, device_numbers=None, device_accounts=None, account_domains=None):
    public_ip = str(ipaddress.ip_address(os.environ["PBX_PUBLIC_IP"]))
    safe(DOMAIN, re.compile(r"^[a-zA-Z0-9.-]+$"))
    trunk_domain = safe(os.environ["TWILIO_SIP_DOMAIN"], re.compile(r"^[a-zA-Z0-9.-]+$"))
    trunk_user = safe(os.environ["PBX_TWILIO_USER"])
    trunk_password = safe(os.environ["PBX_TWILIO_PASSWORD"], PASSWORD)
    incoming_user = safe(os.environ["GOSIP_PBX_TRUNK_USER"])
    incoming_password = safe(os.environ["GOSIP_PBX_TRUNK_PASSWORD"], PASSWORD)
    device_numbers = device_numbers or {}
    device_accounts = device_accounts or {}
    account_domains = account_domains or {}
    primary_sid = os.getenv("TWILIO_ACCOUNT_SID", "")
    account_domains.setdefault(primary_sid, trunk_domain)
    device_passwords = json.loads(os.getenv("PBX_TWILIO_DEVICE_PASSWORDS") or "{}")
    credential_file = STATE / "credentials.json"
    if credential_file.exists():
        device_passwords.update(json.loads(credential_file.read_text()))
    trunk_password = safe(device_passwords.get(trunk_user, trunk_password), PASSWORD)
    pjsip = f"""[global]
type=global
user_agent=Leadomi SIP-Asterisk
endpoint_identifier_order=ip,username
default_realm=gosip
max_forwards=70

[tls]
type=transport
protocol=tls
bind=0.0.0.0:5061
cert_file=/certs/fullchain.pem
priv_key_file=/certs/privkey.pem
ca_list_file=/etc/ssl/certs/ca-certificates.crt
method=tlsv1_2
verify_server=yes
allow_wildcard_certs=yes
verify_client=no
require_client_cert=no
allow_reload=yes
external_signaling_address={public_ip}
external_media_address={public_ip}
local_net=172.16.0.0/12
local_net=10.0.0.0/8
local_net=192.168.0.0/16

[twilio-out-auth]
type=auth
auth_type=userpass
username={trunk_user}
password={trunk_password}

[twilio-out]
type=endpoint
transport=tls
context=deny
media_address={public_ip}
aors=twilio-out
disallow=all
allow=ulaw,alaw
outbound_auth=twilio-out-auth
from_user={trunk_user}
from_domain={trunk_domain}
direct_media=no
rtp_symmetric=yes
force_rport=yes
media_encryption=sdes
media_encryption_optimistic=no
dtmf_mode=rfc4733

[twilio-out]
type=aor
contact=sip:{trunk_domain}:5061\\;transport=tls\\;secure=true

[twilio-in-auth]
type=auth
auth_type=userpass
realm=gosip
username={incoming_user}
password={incoming_password}

[twilio-in]
type=endpoint
transport=tls
context=from-twilio
media_address={public_ip}
message_context=deny
auth=twilio-in-auth
identify_by=ip
disallow=all
allow=ulaw,alaw
direct_media=no
rtp_symmetric=yes
force_rport=yes
media_encryption=sdes
media_encryption_optimistic=no
dtmf_mode=rfc4733

[twilio-identify]
type=identify
endpoint=twilio-in
"""
    # Source networks from Twilio SIP IP documentation. Digest is also required.
    networks = os.environ.get("PBX_TWILIO_NETWORKS", "").split(",")
    for network in networks:
        if network.strip():
            pjsip += "match=" + str(ipaddress.ip_network(network.strip())) + "\n"
    dialplan = f"""[general]
static=yes
writeprotect=yes
clearglobalvars=no

[deny]
exten => _.,1,Hangup(21)

[from-twilio]
"""
    for device in devices:
        username = safe(device["username"])
        identifier = int(device["id"])
        digest = safe(device["password_hash"], re.compile(r"^[a-fA-F0-9]{32}$"))
        outgoing = "twilio-out"
        account_sid = device_accounts.get(username, primary_sid)
        device_domain = safe(account_domains.get(account_sid, trunk_domain), re.compile(r"^[a-zA-Z0-9.-]+$"))
        if username in device_numbers and username != trunk_user:
            outgoing = f"twilio-out-{identifier}"
            outgoing_password = safe(device_passwords[username], PASSWORD)
            pjsip += f"""
[{outgoing}-auth]
type=auth
auth_type=userpass
username={username}
password={outgoing_password}

[{outgoing}]
type=aor
contact=sip:{device_domain}:5061\\;transport=tls\\;secure=true

[{outgoing}]
type=endpoint
transport=tls
context=deny
media_address={public_ip}
aors={outgoing}
disallow=all
allow=ulaw,alaw
outbound_auth={outgoing}-auth
from_user={username}
from_domain={device_domain}
direct_media=no
rtp_symmetric=yes
force_rport=yes
media_encryption=sdes
media_encryption_optimistic=no
dtmf_mode=rfc4733
"""
        pjsip += f"""
[{username}-auth]
type=auth
auth_type=md5
username={username}
realm=gosip
md5_cred={digest}

[{username}]
type=aor
max_contacts=1
remove_existing=yes
minimum_expiration=60
default_expiration=300
maximum_expiration=600
qualify_frequency=30
qualify_timeout=5

[{username}]
type=endpoint
transport=tls
context=from-phone-{identifier}
media_address={public_ip}
message_context=sms-{identifier}
auth={username}-auth
aors={username}
identify_by=username
disallow=all
allow=ulaw,alaw
direct_media=no
rtp_symmetric=yes
force_rport=yes
rewrite_contact=yes
media_encryption=sdes
media_encryption_optimistic=no
dtmf_mode=rfc4733
allow_subscribe=no
rtp_timeout=90
rtp_keepalive=20
"""
        dialplan += f"exten => {username},1,Dial(PJSIP/{username},30)\n same => n,Hangup()\n"
    dialplan += "exten => _.,1,Hangup(21)\n"
    for device in devices:
        username = safe(device["username"])
        identifier = int(device['id'])
        caller_id = safe(device_numbers.get(username, os.environ['GOSIP_OUTBOUND_CALLER_ID']), NUMBER)
        outgoing = f"twilio-out-{identifier}" if username in device_numbers and username != trunk_user else "twilio-out"
        dialplan += f"""
[from-phone-{identifier}]
; Validate the complete E.164 string in the catch-all so leading '+' handling
; is independent of Asterisk's extension-pattern matcher.
exten => _.,1,GotoIf($[${{REGEX("^[+][1-9][0-9]{{7,14}}$" ${{EXTEN}})}} = 1]?valid:invalid)
 same => n(valid),Set(GROUP()=phone-{identifier})
 same => n,GotoIf($[${{GROUP_COUNT(phone-{identifier})}} > 2]?busy)
 same => n,Set(CALLERID(num)={caller_id})
 same => n,Verbose(1,Leadomi outbound attempt on phone account {identifier})
 same => n,Dial(PJSIP/${{EXTEN}}@{outgoing},60)
 same => n,Verbose(1,Leadomi outbound result status=${{DIALSTATUS}} cause=${{HANGUPCAUSE}})
 same => n,Hangup()
 same => n(busy),Hangup(17)
 same => n(invalid),Hangup(28)

[sms-{int(device['id'])}]
exten => _+ZXXXXXXX.,1,Set(GOSIP_SMS_BODY=${{BASE64_ENCODE(${{MESSAGE(body)}})}})
 same => n,AGI(agi://127.0.0.1:4573/sms,{username},${{EXTEN}})
 same => n,Hangup()
exten => _.,1,Hangup(28)
"""
    return pjsip, dialplan


def configure():
    with source() as conn:
        devices = list(conn.execute("SELECT id,username,password_hash FROM devices ORDER BY id"))
        if conn.execute("SELECT 1 FROM sqlite_master WHERE name='sip_accounts'").fetchone():
            inactive = {row[0] for row in conn.execute("SELECT device_id FROM sip_accounts WHERE enabled=0 OR state<>'ready'")}
            devices = [d for d in devices if d['id'] not in inactive]
        row = conn.execute("SELECT value FROM config WHERE key='pbx_device_numbers'").fetchone()
        device_numbers = json.loads(row[0]) if row else {}
        if not isinstance(device_numbers, dict):
            raise ValueError("Phone number assignments must be an object")
        row = conn.execute("SELECT value FROM config WHERE key='pbx_device_accounts'").fetchone()
        device_accounts = json.loads(row[0]) if row else {}
        if not isinstance(device_accounts, dict):
            raise ValueError("Phone account assignments must be an object")
        primary = conn.execute("SELECT value FROM config WHERE key='pbx_primary_twilio_account_sid'").fetchone()
        if primary and primary[0]:
            primary_sid = primary[0]
        account_domains = {}
        for key, value in conn.execute("SELECT key,value FROM config WHERE key LIKE 'pbx_twilio_domain_name_%'"):
            account_domains[key.removeprefix("pbx_twilio_domain_name_")] = value
        if primary_sid:
            account_domains[primary_sid] = os.environ["TWILIO_SIP_DOMAIN"]
        for username in device_numbers:
            account_sid = device_accounts.get(username, primary_sid)
            if account_sid and account_sid not in account_domains:
                raise ValueError("Assigned Twilio account has no configured SIP domain")
    pjsip, dialplan = render(devices, device_numbers, device_accounts, account_domains)
    digest = hashlib.sha256((pjsip + dialplan).encode()).hexdigest()
    old = STATE / "config.sha256"
    if old.exists() and old.read_text() == digest and (CONF / "pjsip.conf").exists():
        return
    atomic(CONF / "pjsip.conf", pjsip)
    atomic(CONF / "extensions.conf", dialplan)
    if PROCESS is not None:
        reload_sip()
        cli("dialplan reload")
    atomic(old, digest)
    LOG.info("SIP configuration synchronized (%d devices)", len(devices))


def ami(action, fields=None, end_event=None):
    with socket.create_connection(("127.0.0.1", 5038), timeout=8) as sock:
        stream = sock.makefile("rwb")
        stream.readline()  # AMI banner

        def send(name, values):
            data = {"Action": name, **values}
            for key, value in data.items():
                if "\r" in str(value) or "\n" in str(value):
                    raise ValueError("Invalid AMI field")
            stream.write(("\r\n".join(k + ": " + str(v) for k, v in data.items()) + "\r\n\r\n").encode())
            stream.flush()

        def frame():
            result = {}
            while True:
                line = stream.readline()
                if not line:
                    raise ConnectionError("AMI closed")
                if line in (b"\r\n", b"\n"):
                    if result:
                        return result
                    continue
                key, separator, value = line.decode(errors="replace").partition(":")
                if separator:
                    result[key] = value.strip()

        send("Login", {"Username": "gosip-bridge", "Secret": AMI_SECRET, "Events": "off"})
        if frame().get("Response") != "Success":
            raise ConnectionError("AMI authentication failed")
        send(action, fields or {})
        frames = []
        while True:
            item = frame()
            frames.append(item)
            if item.get("Response") == "Error":
                if action == "PJSIPShowContacts" and item.get("Message") == "No Contacts found":
                    return []
                raise RuntimeError("AMI action rejected")
            if (end_event and item.get("Event") == end_event) or (not end_event and item.get("Response")):
                return frames


def contacts():
    return {item.get("EndpointName", item.get("Endpoint", ""))
            for item in ami("PJSIPShowContacts", end_event="ContactListComplete")
            if item.get("Event") == "ContactList" and item.get("Status") != "Unreachable"}


def deliver(username, sender, body):
    safe(username)
    safe(sender, NUMBER)
    ami("MessageSend", {"Destination": "pjsip:" + username,
                        "From": "sip:" + sender + "@" + DOMAIN,
                        "Base64Body": base64.b64encode(body.encode()).decode()})


def targets(conn, did):
    ids = set()
    for route in conn.execute("SELECT action_data FROM routes WHERE did_id=? AND enabled=1 AND action_type='ring'", (did,)):
        ids.update(json.loads(route[0]).get("devices", []))
    return [row[1] for row in conn.execute("SELECT id,username FROM devices") if row[0] in ids]


def enqueue(queue):
    with source() as conn:
        cursor = int(queue.execute("SELECT value FROM meta WHERE key='cursor'").fetchone()[0])
        latest = conn.execute("SELECT COALESCE(MAX(id),0) FROM messages").fetchone()[0]
        if latest < cursor:
            # An emptied/restored inbox can reuse IDs. Resume from its beginning
            # instead of silently waiting for it to reach the old high-water mark.
            cursor = 0
            LOG.info("Inbox message IDs restarted; resynchronizing SMS delivery")
        for msg in conn.execute("SELECT id,did_id,from_number,body,media_urls FROM messages WHERE direction='inbound' AND id>? ORDER BY id LIMIT 100", (cursor,)):
            body = msg["body"] or ""
            # Go marshals a nil media slice as JSON null for ordinary SMS.
            for url in json.loads(msg["media_urls"] or "[]") or []:
                body += "\n" + url
            if NUMBER.fullmatch(msg["from_number"]):
                for username in targets(conn, msg["did_id"]):
                    queue.execute("INSERT INTO deliveries(message_id,username,sender,body) VALUES(?,?,?,?) "
                                  "ON CONFLICT(message_id,username) DO UPDATE SET "
                                  "sender=excluded.sender,body=excluded.body,delivered=0 "
                                  "WHERE deliveries.sender<>excluded.sender OR deliveries.body<>excluded.body",
                                  (msg["id"], username, msg["from_number"], body))
            cursor = msg["id"]
        queue.execute("UPDATE meta SET value=? WHERE key='cursor'", (str(cursor),))
        queue.commit()


class AGI(socketserver.StreamRequestHandler):
    def handle(self):
        self.connection.settimeout(15)
        env = {}
        while True:
            line = self.rfile.readline().decode().strip()
            if not line:
                break
            key, _, value = line.partition(": ")
            env[key] = value
        username, number = env.get("agi_arg_1", ""), env.get("agi_arg_2", "")
        try:
            safe(username)
            safe(number, NUMBER)
            self.wfile.write(b"GET VARIABLE GOSIP_SMS_BODY\n")
            self.wfile.flush()
            reply = self.rfile.readline().decode().strip()
            match = re.fullmatch(r"200 result=1 \((.*)\)", reply)
            if not match:
                raise ValueError("Missing SMS body")
            body = base64.b64decode(match[1], validate=True).decode("utf-8")
            request = urllib.request.Request(GOSIP + "/api/pbx/messages",
                data=json.dumps({"username": username, "to_number": number, "body": body}).encode(),
                headers={"Authorization": "Bearer " + SECRET, "Content-Type": "application/json"})
            with urllib.request.urlopen(request, timeout=12) as response:
                if response.status != 202:
                    raise RuntimeError("SMS rejected")
                message_id = int(json.load(response)["id"])
            with sqlite3.connect(STATE / "delivery.db", timeout=5) as queue:
                queue.execute("INSERT OR IGNORE INTO outbound(message_id,username,destination) VALUES(?,?,?)", (message_id, username, number))
            self.wfile.write(b'SET VARIABLE GOSIP_SMS_ACCEPTED "1"\n')
            self.wfile.flush()
            self.rfile.readline()
        except Exception:
            LOG.warning("SMS bridge request failed (message contents omitted)")
            try:
                deliver(username, os.environ["GOSIP_OUTBOUND_CALLER_ID"],
                        "SMS to " + number + " could not be queued. Check Leadomi SIP and try again.")
            except Exception:
                pass


class HTTP(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        if not SECRET or not hmac.compare_digest(self.headers.get("Authorization", ""), "Bearer " + SECRET):
            self.send_error(403)
            return
        if self.path != "/credentials":
            self.send_error(404)
            return
        try:
            length = int(self.headers.get('Content-Length', '0'))
            if not 0 < length <= 4096:
                raise ValueError('Invalid request size')
            request = json.loads(self.rfile.read(length))
            username = safe(request['username'])
            password = safe(request['password'], PASSWORD)
            if not 16 <= len(password) <= 128:
                raise ValueError('Invalid credential')
            with source() as conn:
                if not conn.execute('SELECT 1 FROM devices WHERE username=?', (username,)).fetchone():
                    raise ValueError('Unknown device')
            with CONFIG_LOCK:
                path = STATE / 'credentials.json'
                credentials = json.loads(path.read_text()) if path.exists() else {}
                credentials[username] = password
                atomic(path, json.dumps(credentials))
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'{"saved":true}')
        except (ValueError, KeyError, TypeError):
            self.send_error(400, 'Invalid credential request')
        except Exception:
            self.send_error(503, 'Credential storage unavailable')

    def do_GET(self):
        if self.path == "/health":
            ok = PROCESS is not None and PROCESS.poll() is None and time.time() - LAST_POLL < 60
            self.send_response(200 if ok else 503)
            self.end_headers()
            self.wfile.write(b"ok" if ok else b"unavailable")
            return
        if not SECRET or not hmac.compare_digest(self.headers.get("Authorization", ""), "Bearer " + SECRET):
            self.send_error(403)
            return
        if self.path == '/backup-status':
            path = STATE / 'backup-status.json'
            body = path.read_bytes() if path.exists() else b'{}'
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(body)
            return
        if self.path != "/registrations":
            self.send_error(404)
            return
        try:
            online = contacts()
            with source() as conn:
                result = [{"device_id": row["id"], "username": row["username"], "online": row["username"] in online}
                          for row in conn.execute("SELECT id,username FROM devices")]
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(result).encode())
        except Exception:
            self.send_error(503)


def main():
    global PROCESS, LAST_POLL
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    os.umask(0o077)
    if len(SECRET) < 32:
        raise SystemExit("A strong GOSIP_PBX_SECRET is required")
    STATE.mkdir(parents=True, exist_ok=True)
    CONF.mkdir(parents=True, exist_ok=True)
    atomic(CONF / "asterisk.conf", "[directories]\nastetcdir=/etc/asterisk\nastmoddir=/usr/lib/asterisk/modules\nastvarlibdir=/var/lib/asterisk\nastdbdir=/var/lib/gosip-pbx\nastkeydir=/var/lib/asterisk\nastdatadir=/usr/share/asterisk\nastagidir=/usr/share/asterisk/agi-bin\nastspooldir=/var/spool/asterisk\nastrundir=/var/run/asterisk\nastlogdir=/var/log/asterisk\n[options]\nverbose=0\ndebug=0\n")
    # Load only the modules needed for SIP, audio and messaging.
    atomic(CONF / "modules.conf", "[modules]\nautoload=yes\nnoload=chan_sip.so\nnoload=chan_iax2.so\nnoload=res_hep.so\nnoload=res_hep_pjsip.so\nnoload=res_hep_rtcp.so\nnoload=res_http_websocket.so\nnoload=res_ari.so\nnoload=cdr_csv.so\nnoload=cdr_manager.so\nnoload=cdr_sqlite3_custom.so\nnoload=cel_sqlite3_custom.so\n")
    atomic(CONF / "logger.conf", "[general]\n[logfiles]\nconsole => error,warning,notice\n")
    atomic(CONF / "rtp.conf", "[general]\nrtpstart=10000\nrtpend=10100\nstrictrtp=yes\n")
    atomic(CONF / "http.conf", "[general]\nenabled=no\n")
    atomic(CONF / "manager.conf", "[general]\nenabled=yes\nbindaddr=127.0.0.1\nport=5038\n[gosip-bridge]\nsecret=" + AMI_SECRET + "\ndeny=0.0.0.0/0.0.0.0\npermit=127.0.0.1/255.255.255.255\nread=system,message,reporting\nwrite=system,message,reporting\n")
    configure()
    queue = sqlite3.connect(STATE / "delivery.db")
    queue.executescript("CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY,value TEXT); CREATE TABLE IF NOT EXISTS deliveries(message_id INTEGER,username TEXT,sender TEXT,body TEXT,delivered INTEGER DEFAULT 0,PRIMARY KEY(message_id,username)); CREATE TABLE IF NOT EXISTS outbound(message_id INTEGER PRIMARY KEY,username TEXT,destination TEXT,notified INTEGER DEFAULT 0);")
    if queue.execute("SELECT 1 FROM meta WHERE key='cursor'").fetchone() is None:
        # Do not replay old inbox contents when the bridge is first installed.
        with source() as conn:
            latest = conn.execute("SELECT COALESCE(MAX(id),0) FROM messages").fetchone()[0]
        queue.execute("INSERT INTO meta VALUES('cursor',?)", (str(latest),))
        queue.commit()
    PROCESS = subprocess.Popen(["asterisk", "-f", "-g", "-v"])
    for sig in (signal.SIGTERM, signal.SIGINT):
        signal.signal(sig, lambda *_: STOP.set())
    agi = socketserver.ThreadingTCPServer(("127.0.0.1", 4573), AGI)
    agi.daemon_threads = True
    http_server = http.server.ThreadingHTTPServer(("0.0.0.0", 8088), HTTP)
    for server in (agi, http_server):
        threading.Thread(target=server.serve_forever, daemon=True).start()
    last_config, last_cert = 0, ""
    try:
        while not STOP.wait(3):
            if PROCESS.poll() is not None:
                raise RuntimeError("Asterisk exited")
            try:
                if time.time() - last_config >= 10:
                    configure()
                    last_config = time.time()
                    cert = hashlib.sha256(Path("/certs/fullchain.pem").read_bytes()).hexdigest()
                    if last_cert and cert != last_cert:
                        reload_sip()
                    last_cert = cert
                enqueue(queue)
                online = contacts()
                with source() as conn:
                    for sent in queue.execute("SELECT message_id,username,destination FROM outbound WHERE notified=0").fetchall():
                        status = conn.execute("SELECT status FROM messages WHERE id=?", (sent[0],)).fetchone()
                        if status and status[0] in ("failed", "undelivered", "delivered"):
                            if status[0] == "delivered":
                                queue.execute("UPDATE outbound SET notified=1 WHERE message_id=?", (sent[0],))
                            elif sent[1] in online:
                                deliver(sent[1], os.environ["GOSIP_OUTBOUND_CALLER_ID"], "SMS to " + sent[2] + " failed to deliver. Check the message status in Leadomi SIP before retrying.")
                                queue.execute("UPDATE outbound SET notified=1 WHERE message_id=?", (sent[0],))
                    queue.commit()
                for msg in queue.execute("SELECT * FROM deliveries WHERE delivered=0 LIMIT 100").fetchall():
                    if msg[1] in online:
                        deliver(msg[1], msg[2], msg[3])
                        queue.execute("UPDATE deliveries SET delivered=1 WHERE message_id=? AND username=?", (msg[0], msg[1]))
                        queue.commit()
                LAST_POLL = time.time()
            except Exception as err:
                LOG.warning("Bridge poll failed (%s)", type(err).__name__)
    finally:
        PROCESS.terminate()
        PROCESS.wait(timeout=20)
        queue.close()


if __name__ == "__main__":
    main()
