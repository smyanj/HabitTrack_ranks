let state = null;
let filter = 'all';
let activeView = 'dashboard';
let selectedSeason = null;
const $ = s => document.querySelector(s);
const habitGrid = $('#habitGrid');
const activityList = $('#activityList');
const habitModal = $('#habitModal');
const settingsModal = $('#settingsModal');
const rankModal = $('#rankModal');

async function api(path, options={}) {
  const res = await fetch(path, {headers:{'Content-Type':'application/json'}, ...options});
  const body = await res.json().catch(()=>({}));
  if (!res.ok) throw new Error(body.error || 'Something went wrong');
  return body;
}

async function load() {
  state = await api('/api/state');
  render();
}

function render() {
  $('#seasonYear').textContent = state.seasonYear;
  updateCountdown();
  $('#welcome').textContent = `${state.settings.playerName}, ready for your next quest?`;
  $('#scoreBig').textContent = state.score.toLocaleString();
  $('#scoreText').textContent = `${state.score.toLocaleString()} XP`;
  $('#actionCount').textContent = state.entries.length;
  $('#rankName').textContent = state.rank.name;
  $('#rankBadge').innerHTML = `<span>${roman(state.rank.name)}</span>`;
  const recentWeek = state.entries.filter(e => Date.now() - new Date(e.created).getTime() < 7*24*3600*1000).length;
  $('#streakLabel').textContent = `${recentWeek} action${recentWeek===1?'':'s'}`;

  if (state.nextRank) {
    const span = state.nextRank.required - state.rank.required;
    const into = Math.max(0, state.score - state.rank.required);
    const pct = Math.max(0, Math.min(100, span ? into/span*100 : 100));
    $('#nextText').textContent = `${Math.max(0,state.nextRank.required-state.score).toLocaleString()} XP to ${state.nextRank.name}`;
    $('#progressBar').style.width = `${pct}%`;
  } else {
    $('#nextText').textContent = 'Max rank achieved';
    $('#progressBar').style.width = '100%';
  }
  renderHabits(); renderActivity(); renderRanks(); renderArchives(); renderSeasons();
  if (activeView === 'seasons') $('#welcome').textContent = 'Your season legacy';
}

function roman(name) {
  const last = name.split(' ').at(-1);
  return ['I','II','III','IV'].includes(last) ? last.replace('IV','Ⅳ').replace('III','Ⅲ').replace('II','Ⅱ').replace('I','Ⅰ') : '★';
}

function renderHabits() {
  const habits = state.habits.filter(h => filter==='all' || (filter==='positive' ? h.points>0 : h.points<0));
  if (!habits.length) { habitGrid.innerHTML = `<div class="empty panel">No habits in this category yet.</div>`; return; }
  habitGrid.innerHTML = habits.map(h => `<article class="habit-card">
    <div class="habit-top"><div class="habit-icon">${esc(h.icon)}</div><div class="habit-actions"><button class="habit-menu" title="Edit habit" onclick="editHabit(${h.id})">✎</button><button class="habit-menu delete-habit" title="Delete habit" aria-label="Delete ${esc(h.name)}" onclick="deleteHabit(${h.id})">✕</button></div></div>
    <h3>${esc(h.name)}</h3><div class="category">${esc(h.category || 'General')}</div>
    <div class="habit-bottom"><span class="points ${h.points>0?'positive':'negative'}">${h.points>0?'+':''}${h.points} XP</span><button class="complete-btn" onclick="logHabit(${h.id}, this)">${h.points>0?'Complete':'Log'}</button></div>
  </article>`).join('');
}

function renderActivity() {
  const habitsById = Object.fromEntries(state.habits.map(h=>[h.id,h]));
  const entries = [...state.entries].sort((a,b)=>new Date(b.created)-new Date(a.created));
  if (!entries.length) { activityList.innerHTML = `<div class="empty">Your quest history will appear here.<br><br>Complete a habit to earn your first XP.</div>`; return; }
  activityList.innerHTML = entries.map(e => `<div class="activity-item">
    <div class="activity-icon">${esc(habitsById[e.habitId]?.icon || '⚡')}</div><div class="activity-copy"><strong>${esc(e.habitName)}</strong><span>${timeAgo(e.created)}</span></div>
    <div class="activity-points ${e.points>0?'positive':'negative'}">${e.points>0?'+':''}${e.points}</div><button class="undo-btn" onclick="undoEntry(${e.id})">undo this action</button>
  </div>`).join('');
}

function renderRanks() {
  $('#rankList').innerHTML = state.ranks.map(r=>`<div class="rank-row ${r.name===state.rank.name?'current':''}"><strong>${r.name}</strong><span>${r.required.toLocaleString()} XP</span></div>`).join('');
}

async function deleteHabit(id) {
  const h = state.habits.find(x=>x.id===id);
  if (!h || !confirm(`Delete "${h.name}"? Previously earned/lost XP and activity history will be kept.`)) return;
  try { await api(`/api/habits/${id}`, {method:'DELETE'}); await load(); toast('Habit deleted'); } catch(e) { toast(e.message); }
}

function updateCountdown() {
  if (!state) return;
  const ms = new Date(state.seasonEnd).getTime() - Date.now();
  if (ms <= 0) { $('#seasonCountdown').textContent = 'Season ending…'; load().catch(e=>toast(e.message)); return; }
  let t = Math.floor(ms/1000);
  const d = Math.floor(t/86400); t %= 86400;
  const h = Math.floor(t/3600); t %= 3600;
  const m = Math.floor(t/60), sec = t%60;
  $('#seasonCountdown').textContent = `${d}d ${String(h).padStart(2,'0')}h ${String(m).padStart(2,'0')}m ${String(sec).padStart(2,'0')}s`;
}

function renderArchives() {
  const archives = [...(state.archives || [])].reverse();
  $('#archiveList').innerHTML = archives.length ? archives.map(a=>`<div class="rank-row"><strong>${a.year}${a.year===state.seasonYear?' (early reset)':''}</strong><span>${a.score.toLocaleString()} XP · ${esc(a.rank)}</span></div>`).join('') : '<p class="form-hint">Finished seasons will appear here.</p>';
}

function showView(view) {
  activeView = view;
  $('#dashboardView').hidden = view !== 'dashboard';
  $('#seasonsView').hidden = view !== 'seasons';
  $('#newHabitBtn').hidden = view !== 'dashboard';
  $('#welcome').textContent = view === 'seasons' ? 'Your season legacy' : `${state.settings.playerName}, ready for your next quest?`;
  document.querySelectorAll('.sidebar nav .nav-item').forEach(b => b.classList.remove('active'));
  $(view === 'seasons' ? '#seasonsNav' : '#dashboardNav').classList.add('active');
  if (view === 'seasons') renderSeasons();
}

function renderSeasons() {
  if (!state) return;
  const archives = [...(state.archives || [])].map((a, index) => ({...a, archiveIndex:index})).reverse();
  const cards = $('#seasonCards');
  const detail = $('#seasonDetail');
  if (!archives.length) {
    cards.innerHTML = '<div class="panel season-empty"><span class="empty-trophy">🏆</span><h3>No completed seasons yet</h3><p>Your final XP and rank will appear here after a yearly reset. Your current season is still in progress.</p></div>';
    detail.innerHTML = '';
    return;
  }
  if (!archives.some(a => a.archiveIndex === selectedSeason)) selectedSeason = archives[0].archiveIndex;
  cards.innerHTML = archives.map(a => `<button class="season-card panel ${a.archiveIndex===selectedSeason?'selected':''}" data-season="${a.archiveIndex}">
    <span class="season-card-label">${a.year} SEASON${archives.filter(x=>x.year===a.year).length>1?' · RESET':''}</span>
    <strong>${esc(a.rank)}</strong><span class="season-card-xp">${a.score.toLocaleString()} XP</span><span class="season-card-meta">${(a.entries||[]).length} logged actions →</span>
  </button>`).join('');
  cards.querySelectorAll('[data-season]').forEach(b=>b.onclick=()=>{selectedSeason=Number(b.dataset.season);renderSeasons();});
  const a = archives.find(x=>x.archiveIndex===selectedSeason);
  const entries = [...(a.entries||[])].sort((x,y)=>new Date(y.created)-new Date(x.created));
  const positives = entries.filter(e=>e.points>0);
  const penalties = entries.filter(e=>e.points<0);
  const totalEarned=positives.reduce((n,e)=>n+e.points,0);
  const totalLost=penalties.reduce((n,e)=>n+Math.abs(e.points),0);
  detail.innerHTML = `<div class="history-stats">
    <div class="panel history-stat"><span>FINAL RANK</span><strong>🏆 ${esc(a.rank)}</strong></div>
    <div class="panel history-stat"><span>FINAL XP</span><strong>${a.score.toLocaleString()}</strong></div>
    <div class="panel history-stat"><span>XP EARNED</span><strong class="positive">+${totalEarned.toLocaleString()}</strong></div>
    <div class="panel history-stat"><span>XP LOST</span><strong class="negative">-${totalLost.toLocaleString()}</strong></div>
  </div><div class="section-heading history-log-heading"><div><p class="eyebrow">SEASON ${a.year}</p><h2>Activity history <span class="history-count">(${entries.length})</span></h2></div></div>
  <div class="panel history-activity">${entries.length ? entries.map(e=>`<div class="history-entry"><span class="history-entry-date">${new Date(e.created).toLocaleDateString()}</span><strong>${esc(e.habitName)}</strong><span class="${e.points>0?'positive':'negative'}">${e.points>0?'+':''}${e.points} XP</span></div>`).join(''):'<div class="empty">No logged actions were recorded for this season.</div>'}</div>`;
}

$('#dashboardNav').onclick = () => showView('dashboard');
$('#seasonsNav').onclick = () => showView('seasons');

async function logHabit(id, btn) {
  try {
    const habit = state.habits.find(h=>h.id===id);
    const rect = btn.getBoundingClientRect();
    scorePop(habit.points, rect.left+rect.width/2, rect.top);
    await api('/api/log', {method:'POST', body:JSON.stringify({habitId:id})});
    await load();
    toast(`${habit.points>0?'Quest complete':'Penalty logged'}: ${habit.points>0?'+':''}${habit.points} XP`);
  } catch(e) { toast(e.message); }
}

async function undoEntry(id) {
  try { await api(`/api/entries/${id}`, {method:'DELETE'}); await load(); toast('Action undone'); } catch(e){ toast(e.message); }
}

function editHabit(id) {
  const h = state.habits.find(x=>x.id===id);
  $('#habitModalTitle').textContent = 'Edit habit'; $('#habitId').value=h.id; $('#habitName').value=h.name; $('#habitPoints').value=h.points; $('#habitCategory').value=h.category; $('#habitIcon').value=h.icon;
  habitModal.showModal();
  setTimeout(()=>$('#habitName').focus(),50);
}

$('#newHabitBtn').onclick = () => { $('#habitModalTitle').textContent='New habit'; $('#habitForm').reset(); $('#habitId').value=''; $('#habitIcon').value='⚡'; habitModal.showModal(); };
$('#settingsBtn').onclick = () => { $('#playerName').value=state.settings.playerName; $('#baseScore').value=state.settings.baseScore; settingsModal.showModal(); };
$('#rankingsNav').onclick = () => rankModal.showModal();
$('#resetSeasonBtn').onclick = async () => {
  if (!confirm('Reset all current season XP to 0? This archives your current score and activity. Your habits will stay.')) return;
  try { await api('/api/reset', {method:'POST'}); settingsModal.close(); await load(); toast('New season started!'); } catch(e) { toast(e.message); }
};
setInterval(updateCountdown, 1000);

document.querySelectorAll('.filter').forEach(b=>b.onclick=()=>{document.querySelectorAll('.filter').forEach(x=>x.classList.remove('active'));b.classList.add('active');filter=b.dataset.filter;renderHabits();});

$('#habitForm').addEventListener('submit', async e => {
  e.preventDefault();
  const id = $('#habitId').value;
  const payload = {name:$('#habitName').value, points:Number($('#habitPoints').value), category:$('#habitCategory').value || 'General', icon:$('#habitIcon').value || '⚡'};
  try { await api(id?`/api/habits/${id}`:'/api/habits',{method:id?'PUT':'POST',body:JSON.stringify(payload)}); habitModal.close(); await load(); toast(id?'Habit updated':'New quest created'); } catch(err){toast(err.message);}
});

$('#settingsForm').addEventListener('submit', async e => {
  e.preventDefault();
  try { await api('/api/settings',{method:'PUT',body:JSON.stringify({playerName:$('#playerName').value,baseScore:Number($('#baseScore').value)})}); settingsModal.close(); await load(); toast('Settings saved'); } catch(err){toast(err.message);}
});

function toast(msg){const t=$('#toast');t.textContent=msg;t.classList.add('show');clearTimeout(window.toastTimer);window.toastTimer=setTimeout(()=>t.classList.remove('show'),2200)}
function scorePop(points,x,y){const d=document.createElement('div');d.className=`score-pop ${points>0?'pos':'neg'}`;d.textContent=`${points>0?'+':''}${points} XP`;d.style.left=`${x}px`;d.style.top=`${y}px`;document.body.appendChild(d);setTimeout(()=>d.remove(),1000)}
function timeAgo(date){const s=Math.floor((Date.now()-new Date(date).getTime())/1000);if(s<60)return'just now';if(s<3600)return`${Math.floor(s/60)}m ago`;if(s<86400)return`${Math.floor(s/3600)}h ago`;return new Date(date).toLocaleDateString(undefined,{month:'short',day:'numeric'});}
function esc(v=''){return String(v).replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));}

load().catch(e=>toast(e.message));
