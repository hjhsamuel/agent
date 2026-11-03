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
