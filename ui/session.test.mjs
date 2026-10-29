import {renderMarkdown} from './dist/markdown.js';
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';
const code=(await readFile(new URL('./dist/app.js',import.meta.url),'utf8')).replace("import './admin.js';",'').replace("import {renderMarkdown} from './markdown.js';",'').replace(/\nstart\(\);/,'');
function setup(status,pages){
 const calls=[],streams=[],timers=[],elements=new Map();
 const element=()=>({children:[],scrollTop:0,get scrollHeight(){return this.children.length*100;},textContent:'',value:'',append(...items){this.children.push(...items);},replaceChildren(...items){this.children=items;},querySelector(){return null;},addEventListener(){},setAttribute(){}});
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
 s.streams[0].handlers.init({data:'abc'});
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
 const stream=s.streams[0];stream.handlers.init({data:'abc'});
 assert.equal(vm.runInContext('thinkingIndicator.hidden',s.context),false);
 stream.handlers.content({data:'**hello',lastEventId:'1'});
 stream.handlers.content({data:' world**',lastEventId:'2'});
 assert.equal(vm.runInContext('answer.markdown',s.context),'**hello world**');
 assert.equal(vm.runInContext('answer.innerHTML',s.context),'<p><strong>hello world</strong></p>');
 assert.equal(vm.runInContext('thinkingIndicator.textContent',s.context),'思考中');
 assert.equal(vm.runInContext('thinkingIndicator.hidden',s.context),false);
 s.setStatus(2);await vm.runInContext('checkState(generation)',s.context);
 assert.equal(vm.runInContext('thinkingIndicator.hidden',s.context),true);
});

test('streamed title immediately adds session and survives list refresh',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 s.streams[0].handlers.title({data:'新标题',lastEventId:'1'});
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
 s.streams[0].handlers.end({data:'',lastEventId:'1'});
 await new Promise(resolve=>setImmediate(resolve));
 assert.ok(!s.streams[0].closed);
 s.setStatus(2);
 s.streams[0].handlers.end({data:'',lastEventId:'2'});
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(s.streams[0].closed,true);
 assert.equal(vm.runInContext("$('status').textContent",s.context),'已完成');
 assert.equal(s.timers.length,0);
});

test('existing session init reconciles completion when end was missed',async()=>{
 const s=setup(1);await vm.runInContext("open('abc')",s.context);
 s.setStatus(2);s.streams[0].handlers.init({data:'abc'});
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(s.streams[0].closed,true);
 assert.equal(s.timers.length,0);
});
