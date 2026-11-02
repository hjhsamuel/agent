import test from 'node:test';
import assert from 'node:assert/strict';
import {toolBody,toolValues} from './dist/tool-form.js';

test('tool details populate credentials and unchanged saves preserve them',()=>{
 for(const credential of [{type:0,key:'X-Api-Key',val:'secret'},{type:1,key:'',val:'token'},{type:0,key:'X-User-ID',val:''}]){
  const values=toolValues({type:2,credential});
  assert.equal(values.authMode,credential.type===0?'header':'bearer');
  assert.equal(values.authKey,credential.key);
  assert.equal(values.authValue,credential.val);
  assert.equal(toolBody(values,2,true,credential).credential,undefined);
  assert.equal(toolBody({...values,authValue:'replacement'},2,true,credential).credential.val,'replacement');
 }
 assert.equal(toolValues({type:2,credential:null}).authMode,'keep');
 assert.equal(toolValues({type:3,credential:{type:0,key:'X-Key',val:'secret'}}).authValue,'');
});

test('default creation uses numeric kind and omits credentials',()=>{
 assert.deepEqual(toolBody({kind:'2',name:'MCP',authMode:'keep',authKey:'',authValue:''},'2'),{kind:2,name:'MCP'});
});
test('editing preserves credentials unless explicitly replaced',()=>{
 assert.deepEqual(toolBody({name:'MCP',agent:'',authMode:'keep',authValue:'stale'},2,true),{name:'MCP'});
 assert.deepEqual(toolBody({authMode:'bearer',authKey:'stale',authValue:'token'},1,true),{credential:{type:1,key:'',val:'token'}});
 assert.deepEqual(toolBody({authMode:'header',authKey:'X-Api-Key',authValue:''},2,true),{credential:{type:0,key:'X-Api-Key',val:''}});
});
test('invalid headers are rejected and local tools omit remote credentials',()=>{
 for(const authKey of ['', 'bad key', 'bad\r\nkey'])assert.throws(()=>toolBody({authMode:'header',authKey},2,true));
 assert.deepEqual(toolBody({authMode:'header',authKey:'',authValue:'secret'},3),{kind:3});
});
