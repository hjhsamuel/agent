export function toolValues(tool={}) {
 const cred=tool.credential;
 const remote=[1,2].includes(Number(tool.type));
 return {name:tool.name||'',url:tool.url||'',kind:tool.type??2,agent:tool.agent_id||'',
  authMode:remote&&cred?.type===0?'header':remote&&cred?.type===1?'bearer':'keep',
  authKey:remote?cred?.key||'':'',authValue:remote?cred?.val||'':''};
}

export function toolBody(data, kind, editing=false, original) {
 const {authMode,authKey,authValue,...body}=data;
 if(!editing)body.kind=Number(kind);
 if(editing&&body.agent==='')delete body.agent;
 if([1,2].includes(Number(kind))&&authMode!=='keep'&&authMode!==undefined){
  if(!['header','bearer'].includes(authMode))throw Error('请选择有效的认证方式');
  if(authMode==='header'&&!/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(authKey||''))throw Error('请输入有效的 Header 名称');
  body.credential={type:authMode==='header'?0:1,key:authMode==='header'?authKey:'',val:authValue||''};
  if(editing&&original&&body.credential.type===original.type&&body.credential.key===(original.type===1?'':original.key||'')&&body.credential.val===(original.val||''))delete body.credential;
 }
 return body;
}
export async function toolAgentOptions(api) {
 const main=await api('/agent/main');
 const options=[],seen=new Set();
 async function visit(agent,path){
  if(seen.has(agent.id))return;
  seen.add(agent.id);options.push([agent.id,path]);
  if(Number(agent.type)===2)return;
  for(const type of Number(agent.type)===0?[1,2]:[2]){
   let offset='';
   const cursors=new Set();
   while(true){
    const page=await api('/agent/'+encodeURIComponent(agent.id)+'/sub?'+new URLSearchParams({type,offset,limit:30}));
    for(const child of page.data||[])await visit(child,path+' / '+child.name);
    if(!page.data?.length||!page.offset||cursors.has(page.offset))break;
    offset=page.offset;cursors.add(offset);
   }
  }
 }
 await visit(main,main.name);
 return options;
}

