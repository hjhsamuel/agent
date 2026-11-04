import {renderMarkdown} from './dist/markdown.js';
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';
const code=(await readFile(new URL('./dist/app.js',import.meta.url),'utf8')).replace("import './admin.js';",'').replace("import {renderMarkdown} from './markdown.js';",'').replace(/\nstart\(\);/,'');
function setup(status,pages){
 const calls=[],streams=[],timers=[],elements=new Map();
 const element=()=>({children:[],scrollTop:0,get scrollHeight(){return this.children.length*100;},textContent:'',value:'',append(...items){for(const item of items)item.parentElement=this;this.children.push(...items);},replaceChildren(...items){this.children=items;},querySelector(){return null;},addEventListener(){},setAttribute(){},before(item){item.parentElement=this.parentElement;this.parentElement.children.splice(this.parentElement.children.indexOf(this),0,item);},after(item){item.parentElement=this.parentElement;this.parentElement.children.splice(this.parentElement.children.indexOf(this)+1,0,item);}});
 const context=vm.createContext({renderMarkdown,document:{getElementById(id){if(!elements.has(id))elements.set(id,element());return elements.get(id);},createElement:element},URLSearchParams,BigInt,setTimeout:(fn,delay)=>{timers.push({fn,delay});return 1;},clearTimeout(){},fetch:async url=>{
  calls.push(url);let data;
  if(url.endsWith('/abc'))data={state:status};
  else if(url.includes('/message?'))data=pages?pages.shift():{data:[{role:'assistant',content:'saved answer'}],offset:1};
  else if(url.endsWith('/input'))data=[];
  else data={data:[],offset:''};
  return {ok:true,json:async()=>({code:200,data})};
 },EventSource:class{constructor(url){this.url=url;this.handlers={};streams.push(this);}addEventListener(name,fn){this.handlers[name]=fn;}close(){this.closed=true;}}});
 vm.runInContext(code,context);return {calls,streams,timers,context,setStatus(value){status=value;}};
}
test('completed session uses history without opening SSE',async()=>{
 const s=setup(2);await vm.runInContext("open('abc','test',1)",s.context);
 assert.equal(s.streams.length,0);assert.ok(s.calls.some(url=>url.includes('/message?')));
});
test('running session uses SSE, ignores stale end, switches to history on completion',async()=>{
 const s=setup(1);await vm.runInContext("open('abc','test',2)",s.context);
 assert.equal(s.streams.length,1);assert.ok(!s.calls.some(url=>url.includes('/message?')));
 await vm.runInContext('checkState(generation)',s.context);
 assert.ok(!s.streams[0].closed);
 s.setStatus(2);await vm.runInContext('checkState(generation)',s.context);
 assert.equal(s.streams[0].closed,true);assert.ok(s.calls.some(url=>url.includes('/message?')));
});

test('new session connects only on first send and waits for init before posting',async()=>{
 const s=setup(2);await vm.runInContext('open()',s.context);
 assert.equal(s.streams.length,0);
 const sending=vm.runInContext("$('prompt').value='**hello**';$('composer').onsubmit({preventDefault(){}})",s.context);
 assert.equal(s.streams.length,1);
 assert.ok(!s.calls.some(url=>url.endsWith('/message')));
 s.streams[0].handlers.init({data:JSON.stringify({data:'abc'})});
 await sending;
 assert.ok(s.calls.includes('/cagent/v1/session/abc/message'));
 assert.equal(vm.runInContext("$('messages').children[0].children[1].innerHTML",s.context),'<p><strong>hello</strong></p>');
});

test('history loads three at a time, starts at bottom and preserves position on prepend',async()=>{
 const msg=content=>({role:'assistant',content});
 const s=setup(2,[{data:[msg('6'),msg('5'),msg('4')],offset:3},{data:[msg('3'),msg('2'),msg('1')],offset:6},{data:[],offset:6}]);
 await vm.runInContext("open('abc')",s.context);
 assert.ok(s.calls.includes('/cagent/v1/session/abc/message?limit=3&offset=0'));
 assert.equal(vm.runInContext("$('messages').scrollTop",s.context),300);
 await vm.runInContext("$('messages').scrollTop=0;Promise.all([loadOlder(),loadOlder()])",s.context);
 assert.equal(s.calls.filter(url=>url.includes('offset=3')).length,1);
 assert.equal(vm.runInContext("$('messages').scrollTop",s.context),300);
 assert.equal(vm.runInContext("$('messages').children.map(el=>el.children[1].markdown).join(',')",s.context),'1,2,3,4,5,6');
 await vm.runInContext('loadOlder()',s.context);
 const count=s.calls.length;
 await vm.runInContext('loadOlder()',s.context);
 assert.equal(s.calls.length,count);
});

test('thinking indicator remains separate from streamed Markdown and hides on completion',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 const stream=s.streams[0];stream.handlers.init({data:JSON.stringify({data:'abc'})});
 assert.equal(vm.runInContext('thinkingIndicator.hidden',s.context),false);
 stream.handlers.content({data:JSON.stringify({data:'**hello'}),lastEventId:'1'});
 stream.handlers.content({data:JSON.stringify({data:' world**'}),lastEventId:'2'});
 assert.equal(vm.runInContext('answer.markdown',s.context),'**hello world**');
 assert.equal(vm.runInContext('answer.innerHTML',s.context),'<p><strong>hello world</strong></p>');
 assert.equal(vm.runInContext('thinkingIndicator.textContent',s.context),'思考中');
 assert.equal(vm.runInContext('thinkingIndicator.hidden',s.context),false);
 s.setStatus(2);await vm.runInContext('checkState(generation)',s.context);
 assert.equal(vm.runInContext('thinkingIndicator.hidden',s.context),true);
});

test('streamed title immediately adds session and survives list refresh',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 s.streams[0].handlers.title({data:JSON.stringify({data:'新标题'}),lastEventId:'1'});
 assert.equal(vm.runInContext("$('title').textContent",s.context),'新标题');
 assert.equal(vm.runInContext("$('sessions').children[0].textContent",s.context),'新标题');
 await vm.runInContext('sessions()',s.context);
 assert.equal(vm.runInContext("$('sessions').children.length",s.context),1);
 assert.equal(vm.runInContext("$('sessions').children[0].className",s.context),'active');
});

test('selection follows session switches and clears for new conversation',async()=>{
 const s=setup(2);
 await vm.runInContext("sessionItems=[{id:'abc',name:'A'},{id:'def',name:'B'}];open('abc')",s.context);
 assert.equal(vm.runInContext("$('sessions').children.map(x=>x.className).join(',')",s.context),'active,');
 await vm.runInContext("open('def')",s.context);
 assert.equal(vm.runInContext("$('sessions').children.map(x=>x.className).join(',')",s.context),',active');
 await vm.runInContext('open()',s.context);
 assert.equal(vm.runInContext("$('sessions').children.map(x=>x.className).join(',')",s.context),',');
});

test('SSE end drives completion without a polling timer and ignores old ends',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 assert.equal(s.timers.length,0);
 s.streams[0].handlers.end({data:JSON.stringify({data:''}),lastEventId:'1'});
 await new Promise(resolve=>setImmediate(resolve));
 assert.ok(!s.streams[0].closed);
 s.setStatus(2);
 s.streams[0].handlers.end({data:JSON.stringify({data:''}),lastEventId:'2'});
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(s.streams[0].closed,true);
 assert.equal(vm.runInContext("$('status').textContent",s.context),'已完成');
 assert.equal(s.timers.length,0);
});

test('existing session init reconciles completion when end was missed',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 s.setStatus(2);s.streams[0].handlers.init({data:JSON.stringify({data:'abc'})});
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(s.streams[0].closed,true);
 assert.equal(s.timers.length,0);
});


test('new reasoning uses a separate block and leaves previous thoughts collapsed',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 const stream=s.streams[0];
 stream.handlers.reasoning({data:JSON.stringify({data:'first thought'}),lastEventId:'1'});
 const first=vm.runInContext('reasoning',s.context);
 assert.equal(first.details.open,true);
 stream.handlers.content({data:JSON.stringify({data:'first answer'}),lastEventId:'2'});
 assert.equal(first.details.open,false);
 const firstAnswer=vm.runInContext('answer',s.context);
 stream.handlers.reasoning({data:JSON.stringify({data:'new thought'}),lastEventId:'3'});
 const second=vm.runInContext('reasoning',s.context);
 assert.notEqual(second,first);
 const box=firstAnswer.box;
 assert.ok(box.children.indexOf(second.details)>box.children.indexOf(firstAnswer));
 assert.ok(box.children.indexOf(second.details)<box.children.indexOf(vm.runInContext('answer',s.context)));
 stream.handlers.content({data:JSON.stringify({data:''}),lastEventId:'3'});
 assert.equal(second.details.open,true);
 assert.equal(second.markdown,'new thought');
 assert.equal(first.markdown,'first thought');
 assert.equal(first.details.open,false);
 stream.handlers.reasoning({data:JSON.stringify({data:' continued'}),lastEventId:'4'});
 assert.equal(vm.runInContext('reasoning',s.context),second);
 assert.equal(second.markdown,'new thought continued');
 assert.equal(first.details.open,false);
 stream.handlers.content({data:JSON.stringify({data:' more answer'}),lastEventId:'5'});
 assert.equal(first.details.open,false);
 assert.equal(second.details.open,false);
 assert.equal(firstAnswer.markdown,'first answer');
 assert.equal(vm.runInContext('answer.markdown',s.context),' more answer');
});

test('empty content events do not collapse active reasoning',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 s.streams[0].handlers.reasoning({data:JSON.stringify({data:'thinking'}),lastEventId:'1'});
 s.streams[0].handlers.content({data:JSON.stringify({data:''}),lastEventId:'2'});
 assert.equal(vm.runInContext('reasoning.details.open',s.context),true);
});


test('each turn creates a fresh stream and closes it on completion',async()=>{
 const s=setup(2);await vm.runInContext("open('abc')",s.context);
 assert.equal(vm.runInContext('connected',s.context),false);
 assert.equal(vm.runInContext("$('send').disabled",s.context),false);
 for(let turn=0;turn<2;turn++){
  s.setStatus(1);
  await vm.runInContext("$('prompt').value='hello';$('composer').onsubmit({preventDefault(){}})",s.context);
  assert.equal(s.streams.length,turn+1);
  const stream=s.streams[turn];
  assert.ok(!stream.closed);
  assert.equal(vm.runInContext("$('messages').children[0].children[1].markdown",s.context),'saved answer');
  stream.handlers.init({data:JSON.stringify({data:'abc'})});
  stream.handlers.content({data:JSON.stringify({data:'answer'}),lastEventId:String(turn*2+1)});
  s.setStatus(2);
  stream.handlers.end({data:JSON.stringify({data:''}),lastEventId:String(turn*2+2)});
  await new Promise(resolve=>setImmediate(resolve));
  assert.equal(stream.closed,true);
  assert.equal(vm.runInContext('source',s.context),undefined);
  assert.equal(vm.runInContext('connected',s.context),false);
  assert.equal(vm.runInContext("$('send').disabled",s.context),false);
  stream.handlers.error({});
  assert.equal(s.timers.length,0);
 }
});

test('reconnect renders restarted sequence IDs and still deduplicates within the stream',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 const old=s.streams[0];
 old.handlers.content({data:JSON.stringify({data:'previous'}),lastEventId:'100'});
 await vm.runInContext("answer=reasoning=undefined;subscribe(current,generation)",s.context);
 const stream=s.streams[1];
 assert.ok(stream.url.includes('seq=100'));
 stream.handlers.init({data:JSON.stringify({data:'abc'})});
 stream.handlers.reasoning({data:JSON.stringify({data:'new thought'}),lastEventId:'1'});
 assert.equal(vm.runInContext('reasoning.markdown',s.context),'new thought');
 stream.handlers.content({data:JSON.stringify({data:'new answer'}),lastEventId:'2'});
 stream.handlers.content({data:JSON.stringify({data:'duplicate'}),lastEventId:'2'});
 old.handlers.content({data:JSON.stringify({data:'stale'}),lastEventId:'101'});
 assert.equal(vm.runInContext('answer.markdown',s.context),'new answer');
 assert.equal(vm.runInContext('answer.innerHTML',s.context),'<p>new answer</p>');
 assert.equal(vm.runInContext('seq',s.context),'2');
 s.setStatus(2);stream.handlers.end({data:JSON.stringify({data:''}),lastEventId:'3'});
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(stream.closed,true);
});

test('JSON reasoning payload preserves whitespace across chunks',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 const chunks=['Let',' me',' think',' ','about',' this.\n  next line'];
 chunks.forEach((data,i)=>s.streams[0].handlers.reasoning({data:JSON.stringify({data}),lastEventId:String(i+1)}));
 assert.equal(vm.runInContext('reasoning.markdown',s.context),chunks.join(''));
 assert.equal(vm.runInContext('decodeStreamData(JSON.stringify({data:JSON.stringify({id:"call",content:" hint"})}))',s.context),JSON.stringify({id:'call',content:' hint'}));
 assert.throws(()=>vm.runInContext('decodeStreamData("plain text")',s.context));
 assert.throws(()=>vm.runInContext('decodeStreamData("{}")',s.context));
});
