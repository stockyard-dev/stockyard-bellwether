package server

import "net/http"

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashHTML))
}

const dashHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Bellwether</title>
<style>
:root{--bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;--rust:#c45d2c;--rl:#e8753a;--leather:#a0845c;--ll:#c4a87a;--cream:#f0e6d3;--cd:#bfb5a3;--cm:#7a7060;--gold:#d4a843;--green:#4a9e5c;--red:#c44040;--blue:#4a7ec4;--mono:'JetBrains Mono',Consolas,monospace;--serif:'Libre Baskerville',Georgia,serif}
*{margin:0;padding:0;box-sizing:border-box}body{background:var(--bg);color:var(--cream);font-family:var(--mono);font-size:13px;line-height:1.6}
a{color:var(--rl);text-decoration:none}a:hover{color:var(--gold)}
.hdr{padding:.6rem 1.2rem;border-bottom:1px solid var(--bg3);display:flex;justify-content:space-between;align-items:center}
.hdr h1{font-family:var(--serif);font-size:1rem}.hdr h1 span{color:var(--rl)}
.hdr-right{display:flex;gap:1rem;align-items:center;font-size:.7rem;color:var(--leather)}.hdr-right b{color:var(--cream)}
.main{max-width:900px;margin:0 auto;padding:1rem 1.2rem}
.btn{font-family:var(--mono);font-size:.72rem;padding:.35rem .7rem;border:1px solid;cursor:pointer;background:transparent;transition:.15s;white-space:nowrap}
.btn-p{border-color:var(--rust);color:var(--rl)}.btn-p:hover{background:var(--rust);color:var(--cream)}
.btn-d{border-color:var(--bg3);color:var(--cm)}.btn-d:hover{border-color:var(--red);color:var(--red)}
.btn-s{border-color:var(--green);color:var(--green)}.btn-s:hover{background:var(--green);color:var(--bg)}
.btn-b{border-color:var(--blue);color:var(--blue)}.btn-b:hover{background:var(--blue);color:var(--cream)}

.overview{display:flex;gap:1.5rem;margin-bottom:1.2rem;font-size:.7rem;color:var(--leather);flex-wrap:wrap}
.overview .stat{text-align:center}.overview .stat b{display:block;font-size:1.4rem;color:var(--cream)}
.stat-up b{color:var(--green)!important}.stat-down b{color:var(--red)!important}

.mon-card{background:var(--bg2);border:1px solid var(--bg3);padding:.8rem;margin-bottom:.5rem;cursor:pointer;transition:background .1s}
.mon-card:hover{background:var(--bg3)}
.mon-top{display:flex;align-items:center;gap:.6rem}
.mon-dot{width:12px;height:12px;border-radius:50%;flex-shrink:0}
.mon-dot.up{background:var(--green)}.mon-dot.down{background:var(--red)}.mon-dot.unknown{background:var(--cm)}.mon-dot.paused{background:var(--gold)}
.mon-name{font-size:.85rem;font-weight:600;flex:1}
.mon-uptime{font-size:.8rem;font-weight:600}
.mon-uptime.good{color:var(--green)}.mon-uptime.warn{color:var(--gold)}.mon-uptime.bad{color:var(--red)}
.mon-meta{font-size:.65rem;color:var(--cm);margin-top:.3rem;margin-left:18px;display:flex;gap:.8rem;flex-wrap:wrap}
.sparkline{display:flex;align-items:flex-end;gap:1px;height:20px;margin-left:18px;margin-top:.3rem}
.spark-bar{width:4px;min-height:1px;border-radius:1px}.spark-up{background:var(--green)}.spark-down{background:var(--red)}

.modal-bg{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.65);display:flex;align-items:center;justify-content:center;z-index:100}
.modal{background:var(--bg2);border:1px solid var(--bg3);padding:1.5rem;width:95%;max-width:700px;max-height:90vh;overflow-y:auto}
.modal h2{font-family:var(--serif);font-size:.95rem;margin-bottom:1rem}
label.fl{display:block;font-size:.65rem;color:var(--leather);text-transform:uppercase;letter-spacing:1px;margin-bottom:.25rem;margin-top:.7rem}
input[type=text],input[type=number],input[type=url],select{background:var(--bg);border:1px solid var(--bg3);color:var(--cream);padding:.4rem .6rem;font-family:var(--mono);font-size:.8rem;width:100%;outline:none}
input:focus,select:focus{border-color:var(--rust)}
.form-row{display:flex;gap:.5rem}.form-row>*{flex:1}
.empty{text-align:center;padding:2rem;color:var(--cm);font-style:italic;font-family:var(--serif)}

.check-row{display:flex;align-items:center;gap:.6rem;padding:.3rem 0;border-bottom:1px solid var(--bg3);font-size:.72rem}
.check-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
.check-dot.up{background:var(--green)}.check-dot.down{background:var(--red)}
.check-time{color:var(--cm);font-size:.6rem}
.check-ms{color:var(--leather)}.check-err{color:var(--red);font-size:.65rem}

.inc-row{padding:.4rem 0;border-bottom:1px solid var(--bg3);font-size:.72rem}
.inc-status{font-weight:600}.inc-ongoing{color:var(--red)}.inc-resolved{color:var(--green)}
</style>
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital@0;1&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
</head><body>
<div class="hdr">
<h1><span>Bellwether</span></h1>
<div class="hdr-right">
<span>Monitors: <b id="sTotal">-</b></span>
<button class="btn btn-p" onclick="showNewMonitor()">+ Monitor</button>
</div>
</div>
<div class="main"><div id="upgrade-banner" style="display:none;background:#241e18;border:1px solid #8b3d1a;border-left:3px solid #c45d2c;padding:.6rem 1rem;font-size:.78rem;color:#bfb5a3;margin-bottom:.8rem"><strong style="color:#f0e6d3">Free tier</strong> — 10 items max. <a href="https://stockyard.dev/bellwether/" target="_blank" style="color:#e8753a">Upgrade to Pro →</a></div>
<div class="overview" id="overview"></div>
<div id="monitors"></div>
</div>
<div id="modal"></div>

<script>
let monitors=[];

async function api(url,opts){const r=await fetch(url,opts);return r.json()}
function esc(s){return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
function timeAgo(d){if(!d)return'never';const s=Math.floor((Date.now()-new Date(d))/1e3);if(s<60)return s+'s ago';if(s<3600)return Math.floor(s/60)+'m ago';if(s<86400)return Math.floor(s/3600)+'h ago';return Math.floor(s/86400)+'d ago'}

async function load(){
  const [md,sd]=await Promise.all([api('/api/monitors'),api('/api/stats')]);
  monitors=md.monitors||[];
  document.getElementById('sTotal').textContent=sd.monitors;
  const ov=document.getElementById('overview');
  ov.innerHTML='<div class="stat stat-up"><b>'+sd.up+'</b>Up</div><div class="stat stat-down"><b>'+sd.down+'</b>Down</div><div class="stat"><b>'+sd.paused+'</b>Paused</div><div class="stat"><b>'+sd.incidents+'</b>Incidents</div>';
  render();
}

async function render(){
  const el=document.getElementById('monitors');
  if(!monitors.length){el.innerHTML='<div class="empty">No monitors yet. Add one to start tracking uptime.</div>';return}

  // fetch sparklines for each monitor
  const sparkData={};
  await Promise.all(monitors.map(async m=>{
    const d=await api('/api/monitors/'+m.id+'/checks?limit=30');
    sparkData[m.id]=(d.checks||[]).reverse();
  }));

  el.innerHTML=monitors.map(m=>{
    const upPct=m.uptime_pct.toFixed(1);
    const pctCls=m.uptime_pct>=99?'good':m.uptime_pct>=95?'warn':'bad';
    const statusCls=m.paused?'paused':m.status;
    const statusLabel=m.paused?'paused':m.status;
    const checks=sparkData[m.id]||[];
    const maxMs=Math.max(1,...checks.map(c=>c.resp_time_ms));
    const spark=checks.map(c=>{
      const h=Math.max(2,Math.round(c.resp_time_ms/maxMs*20));
      return '<div class="spark-bar spark-'+c.status+'" style="height:'+h+'px" title="'+c.resp_time_ms+'ms"></div>'
    }).join('');
    return '<div class="mon-card" onclick="showDetail(\''+m.id+'\')">'+
      '<div class="mon-top">'+
        '<div class="mon-dot '+statusCls+'"></div>'+
        '<div class="mon-name">'+esc(m.name)+'</div>'+
        '<div class="mon-uptime '+pctCls+'">'+upPct+'%</div>'+
      '</div>'+
      '<div class="sparkline">'+spark+'</div>'+
      '<div class="mon-meta">'+
        '<span>'+esc(m.url)+'</span>'+
        '<span>'+m.type.toUpperCase()+'</span>'+
        '<span>every '+m.interval_sec+'s</span>'+
        (m.last_resp_ms?'<span>'+m.last_resp_ms+'ms</span>':'')+
        '<span>checked '+timeAgo(m.last_check_at)+'</span>'+
        '<span>'+m.check_count+' checks</span>'+
      '</div></div>'
  }).join('')
}

async function showDetail(id){
  const [m,cd,inc]=await Promise.all([api('/api/monitors/'+id),api('/api/monitors/'+id+'/checks?limit=50'),api('/api/monitors/'+id+'/incidents?limit=20')]);
  const checks=(cd.checks||[]).map(c=>'<div class="check-row"><div class="check-dot '+c.status+'"></div><span class="check-ms">'+c.resp_time_ms+'ms</span>'+(c.status_code?'<span style="color:var(--leather)">HTTP '+c.status_code+'</span>':'')+
    (c.error_msg?'<span class="check-err">'+esc(c.error_msg)+'</span>':'')+
    '<span class="check-time" style="margin-left:auto">'+timeAgo(c.created_at)+'</span></div>').join('');
  const incidents=(inc.incidents||[]).map(i=>'<div class="inc-row"><span class="inc-status '+(i.resolved_at?'inc-resolved':'inc-ongoing')+'">'+
    (i.resolved_at?'Resolved':'Ongoing')+'</span> '+esc(i.cause)+' <span style="color:var(--cm);font-size:.6rem">'+i.duration+' · started '+timeAgo(i.started_at)+'</span></div>').join('');
  const pctCls=m.uptime_pct>=99?'good':m.uptime_pct>=95?'warn':'bad';
  const pauseBtn=m.paused?'<button class="btn btn-s" onclick="resumeMon(\''+id+'\')">Resume</button>':'<button class="btn btn-d" onclick="pauseMon(\''+id+'\')">Pause</button>';
  document.getElementById('modal').innerHTML='<div class="modal-bg" onclick="if(event.target===this)closeModal()"><div class="modal">'+
    '<div style="display:flex;justify-content:space-between;align-items:flex-start">'+
      '<h2><div class="mon-dot '+m.status+'" style="display:inline-block;vertical-align:middle;margin-right:.4rem"></div>'+esc(m.name)+'</h2>'+
      '<div style="display:flex;gap:.3rem">'+
        '<button class="btn btn-b" onclick="triggerCheck(\''+id+'\')">Check now</button>'+
        '<button class="btn btn-p" onclick="editMon(\''+id+'\')">Edit</button>'+
        pauseBtn+
        '<button class="btn btn-d" onclick="if(confirm(\'Delete?\'))delMon(\''+id+'\')">Del</button>'+
      '</div>'+
    '</div>'+
    '<div style="display:flex;gap:1.5rem;margin:.6rem 0;font-size:.7rem;color:var(--leather);flex-wrap:wrap">'+
      '<span>URL: '+esc(m.url)+'</span>'+
      '<span>Type: '+m.type.toUpperCase()+'</span>'+
      '<span>Interval: '+m.interval_sec+'s</span>'+
      '<span>Timeout: '+m.timeout_sec+'s</span>'+
      '<span class="mon-uptime '+pctCls+'">Uptime: '+m.uptime_pct.toFixed(2)+'%</span>'+
      '<span>'+m.incident_count+' incidents</span>'+
    '</div>'+
    '<div style="font-size:.7rem;color:var(--leather);margin:1rem 0 .3rem">Recent checks ('+m.check_count+' total)</div>'+
    (checks||'<div style="font-size:.75rem;color:var(--cm)">No checks yet.</div>')+
    '<div style="font-size:.7rem;color:var(--leather);margin:1rem 0 .3rem">Incidents</div>'+
    (incidents||'<div style="font-size:.75rem;color:var(--cm)">No incidents.</div>')+
  '</div></div>'
}

async function triggerCheck(id){
  await api('/api/monitors/'+id+'/check',{method:'POST'});
  showDetail(id);load()
}
async function pauseMon(id){await api('/api/monitors/'+id+'/pause',{method:'POST'});closeModal();load()}
async function resumeMon(id){await api('/api/monitors/'+id+'/resume',{method:'POST'});closeModal();load()}
async function delMon(id){await api('/api/monitors/'+id,{method:'DELETE'});closeModal();load()}

function showNewMonitor(){
  document.getElementById('modal').innerHTML='<div class="modal-bg" onclick="if(event.target===this)closeModal()"><div class="modal">'+
    '<h2>New Monitor</h2>'+
    '<label class="fl">Name</label><input type="text" id="nm-name" placeholder="My API">'+
    '<label class="fl">URL / Host</label><input type="text" id="nm-url" placeholder="https://example.com/health">'+
    '<div class="form-row">'+
      '<div><label class="fl">Type</label><select id="nm-type"><option value="http">HTTP</option><option value="tcp">TCP</option><option value="dns">DNS</option></select></div>'+
      '<div><label class="fl">Method</label><select id="nm-method"><option>GET</option><option>HEAD</option><option>POST</option></select></div>'+
    '</div>'+
    '<div class="form-row">'+
      '<div><label class="fl">Interval (sec)</label><input type="number" id="nm-int" value="300"></div>'+
      '<div><label class="fl">Timeout (sec)</label><input type="number" id="nm-timeout" value="10"></div>'+
      '<div><label class="fl">Expected status</label><input type="number" id="nm-exp" value="200"></div>'+
    '</div>'+
    '<div style="display:flex;gap:.5rem;margin-top:1rem"><button class="btn btn-p" onclick="saveNewMon()">Create</button><button class="btn btn-d" onclick="closeModal()">Cancel</button></div>'+
  '</div></div>'
}

async function saveNewMon(){
  const body={
    name:document.getElementById('nm-name').value,
    url:document.getElementById('nm-url').value,
    type:document.getElementById('nm-type').value,
    method:document.getElementById('nm-method').value,
    interval_sec:parseInt(document.getElementById('nm-int').value)||300,
    timeout_sec:parseInt(document.getElementById('nm-timeout').value)||10,
    expected_status:parseInt(document.getElementById('nm-exp').value)||200
  };
  if(!body.name||!body.url){alert('Name and URL required');return}
  const r=await api('/api/monitors',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(r.error){alert(r.error);return}
  closeModal();load()
}

function editMon(id){
  const m=monitors.find(x=>x.id===id);if(!m)return;
  document.getElementById('modal').innerHTML='<div class="modal-bg" onclick="if(event.target===this)closeModal()"><div class="modal">'+
    '<h2>Edit Monitor</h2>'+
    '<label class="fl">Name</label><input type="text" id="em-name" value="'+esc(m.name)+'">'+
    '<label class="fl">URL / Host</label><input type="text" id="em-url" value="'+esc(m.url)+'">'+
    '<div class="form-row">'+
      '<div><label class="fl">Type</label><select id="em-type"><option value="http"'+(m.type==='http'?' selected':'')+'>HTTP</option><option value="tcp"'+(m.type==='tcp'?' selected':'')+'>TCP</option><option value="dns"'+(m.type==='dns'?' selected':'')+'>DNS</option></select></div>'+
      '<div><label class="fl">Method</label><select id="em-method"><option'+(m.method==='GET'?' selected':'')+'>GET</option><option'+(m.method==='HEAD'?' selected':'')+'>HEAD</option><option'+(m.method==='POST'?' selected':'')+'>POST</option></select></div>'+
    '</div>'+
    '<div class="form-row">'+
      '<div><label class="fl">Interval (sec)</label><input type="number" id="em-int" value="'+m.interval_sec+'"></div>'+
      '<div><label class="fl">Timeout (sec)</label><input type="number" id="em-timeout" value="'+m.timeout_sec+'"></div>'+
      '<div><label class="fl">Expected status</label><input type="number" id="em-exp" value="'+m.expected_status+'"></div>'+
    '</div>'+
    '<div style="display:flex;gap:.5rem;margin-top:1rem"><button class="btn btn-p" onclick="saveEditMon(\''+id+'\')">Save</button><button class="btn btn-d" onclick="showDetail(\''+id+'\')">Cancel</button></div>'+
  '</div></div>'
}

async function saveEditMon(id){
  const body={
    name:document.getElementById('em-name').value,
    url:document.getElementById('em-url').value,
    type:document.getElementById('em-type').value,
    method:document.getElementById('em-method').value,
    interval_sec:parseInt(document.getElementById('em-int').value)||300,
    timeout_sec:parseInt(document.getElementById('em-timeout').value)||10,
    expected_status:parseInt(document.getElementById('em-exp').value)||200
  };
  await api('/api/monitors/'+id,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  closeModal();load()
}

function closeModal(){document.getElementById('modal').innerHTML=''}
load();setInterval(load,15000)
fetch('/api/tier').then(r=>r.json()).then(j=>{if(j.tier==='free'){var b=document.getElementById('upgrade-banner');if(b)b.style.display='block'}}).catch(()=>{var b=document.getElementById('upgrade-banner');if(b)b.style.display='block'});
</script></body></html>`
