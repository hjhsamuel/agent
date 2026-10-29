import {renderMarkdown} from './markdown.js';
const $ = id => document.getElementById(id);
let current='', source, seq='', generation=0, busy=false, connected=false, answer, reasoning, sessionOffset='', historyOffset=0;
let transcriptEpoch=0, stateTimer, pendingConnection, thinkingIndicator;
const historyLimit=3;
let historyMore=false, historyLoading;
const api='/cagent/v1/session';
function notice(message=''){ $('notice').textContent=message; $('notice').hidden=!message; }
function state(text){
  $('status').textContent=text; $('send').disabled=busy||(!!current&&!connected);
  const running=busy&&['正在回复…','正在思考…'].includes(text);
  if(thinkingIndicator)thinkingIndicator.hidden=!running;
  if(running){
    answer??=message('assistant');
    if(!thinkingIndicator||thinkingIndicator.message!==answer){
      if(thinkingIndicator)thinkingIndicator.hidden=true;
      thinkingIndicator=document.createElement('div');
      thinkingIndicator.className='thinking-indicator';
      thinkingIndicator.textContent='思考中';
      thinkingIndicator.setAttribute('role','status');
      thinkingIndicator.message=answer;
      answer.box.append(thinkingIndicator);
    }
    thinkingIndicator.hidden=false;
  }
}
async function request(path='',method='GET',body){
  const response=await fetch(api+path,{method,credentials:'same-origin',headers:body?{'Content-Type':'application/json'}:{},body:body?JSON.stringify(body):undefined});
  const result=await response.json();
  if(!response.ok||result.code!==200) throw Error(response.status===401?'认证失败，请在「连接与认证」中设置有效 JWT。':result.message||'请求失败');
  return result.data;
}
function message(role,content=''){
  $('messages').querySelector('.welcome')?.remove();
  const box=document.createElement('article');box.className='message '+(role==='user'?'user':'assistant');
  const name=document.createElement('strong');name.textContent=role==='user'?'你':'CAgent';
  const text=document.createElement('div');text.className='markdown-preview message-content';text.markdown=content;text.innerHTML=renderMarkdown(content);text.box=box;box.append(name,text);$('messages').append(box);return text;
}
function nearBottom(){const el=$('messages');return el.scrollHeight-el.scrollTop-el.clientHeight<220;}
function scroll(follow){if(follow){const el=$('messages');el.scrollTop=el.scrollHeight;}}
function scheduleStateCheck(version){
  clearTimeout(stateTimer);
  stateTimer=setTimeout(async()=>{
    if(version!==generation)return;
    try{await checkState(version);}catch(error){notice(error.message);}
    if(version===generation&&source)scheduleStateCheck(version);
  },3000);
}
async function checkState(version){
  const id=current, epoch=transcriptEpoch;
  const info=await request('/'+id);
  if(version!==generation||epoch!==transcriptEpoch)return;
  if(info.state!==2)return;
  source?.close();source=undefined;clearTimeout(stateTimer);
  // Invalidate callbacks already queued by the closed EventSource.
  const nextVersion=++generation;
  busy=true;connected=false;state('正在加载最近消息…');
  try{await history(id,nextVersion);if(nextVersion!==generation)return;
    busy=false;connected=true;answer=reasoning=undefined;$('inputs').replaceChildren();state('已完成');
    await sessions();
  }catch(error){if(nextVersion===generation){notice(error.message);state('历史加载失败，请重新打开会话');}}
}
async function sessions(more=false){
  const data=await request('?limit=30'+(more?'&offset='+encodeURIComponent(sessionOffset):''));
  if(!more)$('sessions').replaceChildren(); sessionOffset=data.offset;
  for(const item of data.data){const button=document.createElement('button');button.textContent=item.name||'未命名对话';button.className=item.id===current?'active':'';button.onclick=()=>open(item.id,item.name,item.state);$('sessions').append(button);}
  $('more').hidden=data.data.length<30;
}
function subscribe(id,version,onReady){
  source?.close();connected=false;state('正在连接…');
  const stream=new EventSource(api+'/stream?'+new URLSearchParams({...(id?{id}:{}),seq}),{withCredentials:true});source=stream;
  const listen=(type,fn)=>stream.addEventListener(type,event=>{
    if(version!==generation)return;
    if(event.lastEventId){if(seq && BigInt(event.lastEventId)<=BigInt(seq))return;seq=event.lastEventId;}
    fn(event);
  });
  listen('init',event=>{current=event.data;connected=true;state(busy?'正在回复…':'已连接');$('delete').hidden=false;onReady?.();});
  listen('content',event=>{const follow=nearBottom();busy=true;state('正在回复…');answer??=message('assistant');answer.markdown+=event.data;answer.innerHTML=renderMarkdown(answer.markdown);scroll(follow);});
  listen('reasoning',event=>{const follow=nearBottom();busy=true;state('正在思考…');answer??=message('assistant');if(!reasoning){const details=document.createElement('details');const summary=document.createElement('summary');summary.textContent='思考过程';reasoning=document.createElement('div');reasoning.className='markdown-preview message-content';reasoning.markdown='';details.append(summary,reasoning);answer.before(details);}reasoning.markdown+=event.data;reasoning.innerHTML=renderMarkdown(reasoning.markdown);scroll(follow);});
  listen('title',event=>{$('title').textContent=event.data;sessions().catch(error=>notice(error.message));});
  listen('hitl',event=>{try{input(JSON.parse(event.data),version);}catch{notice('无法解析补充输入请求。');}});
  listen('end',()=>{answer=reasoning=undefined;checkState(version).catch(error=>notice(error.message));});
  stream.addEventListener('error',event=>{
    if(version!==generation)return;
    if(pendingConnection){const reject=pendingConnection;pendingConnection=undefined;stream.close();source=undefined;reject(Error('连接失败，请检查认证和网络后重试。'));return;}
    if(typeof event.data==='string'){notice(event.data);busy=false;answer=reasoning=undefined;state('回复失败');return;}
    stream.close();connected=false;state('连接已断开');notice('连接已断开，3 秒后尝试恢复。认证失败时请更新 JWT。');
    if(current)setTimeout(()=>{if(version===generation)subscribe(current,version);},3000);
  });
}
function input(item,version){
  if([...$('inputs').children].some(el=>el.dataset.id===item.id))return;
  busy=true;state('等待补充输入');
  const form=document.createElement('form');form.dataset.id=item.id;
  const hint=document.createElement('p');hint.textContent=item.content;const field=document.createElement('input');field.required=true;field.setAttribute('aria-label','补充内容');const button=document.createElement('button');button.textContent='提交补充';form.append(hint,field,button);
  form.onsubmit=async event=>{event.preventDefault();button.disabled=true;try{await request('/'+current+'/input','POST',{call_id:item.id,content:field.value});if(version!==generation)return;form.remove();state('正在回复…');}catch(error){notice(error.message);button.disabled=false;}};
  $('inputs').append(form);
}
async function history(id,version,older=false){
  if(older&&(!historyMore||historyLoading))return;
  const epoch=transcriptEpoch, token={};historyLoading=token;
  try{
    const data=await request('/'+id+'/message?limit='+historyLimit+'&offset='+(older?historyOffset:0));
    if(version!==generation||epoch!==transcriptEpoch)return;
    const el=$('messages'),height=el.scrollHeight,top=el.scrollTop;
    const existing=older?[...el.children]:[];
    historyOffset=data.offset;historyMore=data.data.length===historyLimit;
    el.replaceChildren();
    for(const item of [...data.data].reverse())message(item.role,item.content);
    if(older){el.append(...existing);el.scrollTop=top+el.scrollHeight-height;}
    else el.scrollTop=el.scrollHeight;
  }finally{if(historyLoading===token)historyLoading=undefined;}
}
function loadOlder(){
  if(!current||busy||!historyMore||historyLoading)return;
  const version=generation;
  return history(current,version,true).catch(error=>{if(version===generation)notice(error.message);});
}
$('messages').addEventListener('scroll',()=>{if($('messages').scrollTop<=0)loadOlder();});
// A short page may not overflow yet; upward wheel input still loads its previous page.
$('messages').addEventListener('wheel',event=>{if(event.deltaY<0&&$('messages').scrollTop<=0)loadOlder();},{passive:true});
async function open(id='',title='新对话',status=2){
  pendingConnection?.(Error('已切换会话'));pendingConnection=undefined;
  const version=++generation;clearTimeout(stateTimer);transcriptEpoch++;source?.close();source=undefined;current=id;seq='';historyOffset=0;historyMore=false;historyLoading=undefined;busy=status===1||status===5;connected=false;answer=reasoning=undefined;
  $('title').textContent=title||'未命名对话';$('messages').replaceChildren();$('inputs').replaceChildren();$('delete').hidden=!id;notice();state('正在加载…');
  try{
    if(id){const info=await request('/'+id);if(version!==generation)return;
      if(info.state===2){await history(id,version);if(version!==generation)return;busy=false;connected=true;state('已完成');return;}
      busy=true;const pending=await request('/'+id+'/input');if(version!==generation)return;pending.forEach(item=>input(item,version));
    }
    if(!id){busy=false;state('请输入消息开始新对话');return;}
    subscribe(id,version);scheduleStateCheck(version);
  }catch(error){if(version===generation){notice(error.message);state('加载失败');}}
}
$('composer').onsubmit=async event=>{
  event.preventDefault();const content=$('prompt').value.trim();if(!content||busy||(!!current&&!connected))return;
  const version=generation;transcriptEpoch++;historyMore=false;historyLoading=undefined;busy=true;answer=reasoning=undefined;state('正在发送…');notice();
  const sent=message('user',content);$('prompt').value='';$('messages').scrollTop=$('messages').scrollHeight;
  try{
    if(!current){await new Promise((resolve,reject)=>{pendingConnection=reject;subscribe('',version,()=>{pendingConnection=undefined;resolve();});});if(version!==generation)return;}
    await request('/'+current+'/message','POST',{content});if(version!==generation)return;
    if(!source){$('messages').replaceChildren();message('user',content);seq='';subscribe(current,version);}
    state('正在回复…');scheduleStateCheck(version);}
  catch(error){if(version===generation){sent.parentElement.remove();$('prompt').value=content;busy=false;state('发送失败');notice(error.message);}}
};
$('prompt').onkeydown=event=>{if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();$('composer').requestSubmit();}};
$('new').onclick=()=>open();$('more').onclick=()=>sessions(true).catch(error=>notice(error.message));
$('delete').onclick=async()=>{if(!current||!confirm('确定删除当前对话？'))return;try{await request('/'+current,'DELETE');await open();await sessions();}catch(error){notice(error.message);}};
$('settings').onclick=()=>$('config').showModal();
$('config').addEventListener('close',()=>{if($('config').returnValue!=='save')return;const token=$('token').value.trim();if(token){if(!/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(token)){notice('JWT 格式无效。');return;}document.cookie=$('cookieName').value+'='+token+'; Path=/; SameSite=Strict';}$('token').value='';start();});
async function start(){try{await sessions();await open();}catch(error){notice(error.message);state('尚未连接');}}
start();

import './admin.js';
