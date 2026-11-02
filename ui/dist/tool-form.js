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
