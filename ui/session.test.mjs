import {renderMarkdown} from './dist/markdown.js';
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';
const code=(await readFile(new URL('./dist/app.js',import.meta.url),'utf8')).replace("import './admin.js';",'').replace("import {renderMarkdown} from './markdown.js';",'').replace(/\nstart\(\);/,'');
function setup(status){
 const calls=[],streams=[],elements=new Map();
 const element=()=>({children:[],textContent:'',value:'',append(...items){this.children.push(...items);},replaceChildren(...items){this.children=items;},querySelector(){return null;},addEventListener(){},setAttribute(){}});
 const context=vm.createContext({renderMarkdown,document:{getElementById(id){if(!elements.has(id))elements.set(id,element());return elements.get(id);},createElement:element},URLSearchParams,BigInt,setTimeout:()=>1,clearTimeout(){},fetch:async url=>{
  calls.push(url);let data;
  if(url.endsWith('/abc'))data={state:status};
  else if(url.includes('/message?'))data={data:[{role:'assistant',content:'saved answer'}],offset:1};
  else if(url.endsWith('/input'))data=[];
  else data={data:[],offset:''};
  return {ok:true,json:async()=>({code:200,data})};
 },EventSource:class{constructor(url){this.url=url;this.handlers={};streams.push(this);}addEventListener(name,fn){this.handlers[name]=fn;}close(){this.closed=true;}}});
 vm.runInContext(code,context);return {calls,streams,context,setStatus(value){status=value;}};
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
