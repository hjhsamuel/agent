import test from 'node:test';
import assert from 'node:assert/strict';
import {modelBody,modelValues} from './dist/model-form.js';
test('extra preserves nested values and supports clearing or omission',()=>{
 assert.deepEqual(modelBody({extra:'{"thinking":{"enabled":true},"n":2}'}).extra,{thinking:{enabled:true},n:2});
 assert.deepEqual(modelBody({extra:'{}'}),{extra:{}});
 assert.deepEqual(modelBody({extra:'  '}),{});
 for(const extra of ['null','[]','true','2','"text"','{']) assert.throws(()=>modelBody({extra}));
});
test('model detail populates editable configuration',()=>{
 const values=modelValues({id:'1',name:'test'},{context_limit:8192,extra:{nested:[1,false]}});
 assert.equal(values.context_limit,8192);
 assert.deepEqual(modelBody(values).extra,{nested:[1,false]});
 assert.equal(modelValues({}, {extra:null}).extra,'{}');
});

test('model mode is numeric and an unspecified mode is not overwritten',()=>{
 for(const mode of [0,1,'0','1']) assert.equal(modelBody({mode}).mode,Number(mode));
 for(const mode of ['',undefined,null]) assert.equal(Object.hasOwn(modelBody({mode}),'mode'),false);
 for(const mode of [2,-1,'responses',' ',true]) assert.throws(()=>modelBody({mode}));
});
test('mode uses returned detail or list value and otherwise stays unchanged',()=>{
 assert.equal(modelValues({},{}).mode,'');
 assert.equal(modelValues({mode:1},{}).mode,1);
 assert.equal(modelValues({mode:1},{mode:0}).mode,0);
 assert.equal(Object.hasOwn(modelBody(modelValues({},{})),'mode'),false);
});

test('model detail returns URL and mode for editing and saving',()=>{
 for(const mode of [0,1]) {
  const values=modelValues({name:'test',url:'old'},{url:'https://example.com/v1',mode,context_limit:8192,extra:{}});
  assert.equal(values.url,'https://example.com/v1');
  assert.equal(values.mode,mode);
  const body=modelBody({...values,mode:String(values.mode)});
  assert.equal(body.url,values.url);
  assert.equal(body.mode,mode);
 }
});
