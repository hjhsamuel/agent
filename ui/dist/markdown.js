// Markdown preview. Raw HTML is always escaped.
const escape = value => value.replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function inline(source) {
  const tokens = /(`+)([^`]+)\1|\[([^\]]+)\]\(([^\s)]+)\)|\*\*([^*]+)\*\*|~~([^~]+)~~|\*([^*]+)\*|__([^_]+)__|(?<![\w])_([^_]+)_(?![\w])/g;
  let result='', end=0;
  for (const match of source.matchAll(tokens)) {
    result+=escape(source.slice(end,match.index));end=match.index+match[0].length;
    if(match[2])result+='<code>'+escape(match[2])+'</code>';
    else if(match[3]) {
      const url=match[4];
      result+=/^(https?:\/\/|mailto:)/i.test(url)?'<a href="'+escape(url)+'" target="_blank" rel="noopener noreferrer">'+escape(match[3])+'</a>':escape(match[0]);
    } else if(match[5]) result+='<strong>'+inline(match[5])+'</strong>';
    else if(match[6]) result+='<del>'+escape(match[6])+'</del>';
    else if(match[8])result+='<strong>'+inline(match[8])+'</strong>';
    else result+='<em>'+inline(match[7]||match[9])+'</em>';
  }
  return result+escape(source.slice(end));
}
export function renderMarkdown(source) {
 const lines=source.replace(/\r\n?/g,'\n').split('\n');let html='',i=0;
 const cells=line=>line.trim().replace(/^\||\|$/g,'').split(/(?<!\\)\|/).map(cell=>cell.trim().replace(/\\\|/g,'|'));
 const divider=line=>line&&line.includes('|')&&cells(line).every(cell=>/^:?-{3,}:?$/.test(cell));
 const block=line=>/^(#{1,6}\s|\s*(?:```|~~~)|>\s?|\s*[-*+]\s|\s*\d+[.)]\s|---\s*$)/.test(line);
 while(i<lines.length){
  const line=lines[i];if(!line.trim()){i++;continue;}
  if(/^\s*(?:```|~~~)/.test(line)){const fence=line.trim().match(/^(`{3,}|~{3,})/)[1];const close=new RegExp('^\\s*'+fence[0]+'{'+fence.length+',}\\s*$');const code=[];i++;while(i<lines.length&&!close.test(lines[i]))code.push(lines[i++]);i++;html+='<pre><code>'+escape(code.join('\n'))+'</code></pre>';continue;}
  if(i+1<lines.length&&divider(lines[i+1])){const headers=cells(line),align=cells(lines[i+1]).map(c=>c.startsWith(':')?(c.endsWith(':')?'center':'left'):(c.endsWith(':')?'right':'left'));const cell=(value,index,tag)=>'<'+tag+' style="text-align:'+align[index]+'">'+inline(value)+'</'+tag+'>';html+='<table><thead><tr>'+headers.map((v,j)=>cell(v,j,'th')).join('')+'</tr></thead><tbody>';i+=2;while(i<lines.length&&lines[i].includes('|')&&lines[i].trim()){const row=cells(lines[i++]);html+='<tr>'+headers.map((_,j)=>cell(row[j]||'',j,'td')).join('')+'</tr>';}html+='</tbody></table>';continue;}
  if(i+1<lines.length&&/^(?:===+|---+)\s*$/.test(lines[i+1])){const tag=lines[i+1][0]==='='?'h1':'h2';html+='<'+tag+'>'+inline(line)+'</'+tag+'>';i+=2;continue;}
  const heading=/^(#{1,6})\s+(.+)$/.exec(line);if(heading){html+=`<h${heading[1].length}>${inline(heading[2])}</h${heading[1].length}>`;i++;continue;}
  if(/^---\s*$/.test(line)){html+='<hr>';i++;continue;}
  if(/^>/.test(line)){const quote=[];while(i<lines.length&&/^>/.test(lines[i]))quote.push(lines[i++].replace(/^>\s?/,''));html+='<blockquote>'+renderMarkdown(quote.join('\n'))+'</blockquote>';continue;}
  const list=/^\s*([-*+]|\d+[.)])\s+/.exec(line);
  if(list){
   const ordered=/\d/.test(list[1]),tag=ordered?'ol':'ul',pattern=ordered?/^(\s*)\d+[.)]\s+/:/^(\s*)[-*+]\s+/;
   const indent=line.match(/^\s*/)[0].length;
   html+='<'+tag+'>';
   while(i<lines.length){const match=pattern.exec(lines[i]);if(!match||match[1].length!==indent)break;
    const body=[lines[i++].slice(match[0].length)];
    while(i<lines.length&&lines[i].trim()&&lines[i].match(/^\s*/)[0].length>indent)body.push(lines[i++].slice(Math.min(match[0].length,lines[i-1].match(/^\s*/)[0].length)));
    let content=renderMarkdown(body.join('\n')).replace(/^<p>([\s\S]*?)<\/p>/,'$1');
    content=content.replace(/^\[([ xX])\]\s*/,(_,checked)=>'<input type="checkbox" disabled'+(checked.toLowerCase()==='x'?' checked':'')+'> ');
    html+='<li>'+content+'</li>';
   }
   html+='</'+tag+'>';continue;
  }
  const paragraph=[line];i++;while(i<lines.length&&lines[i].trim()&&!block(lines[i])&&!divider(lines[i+1]))paragraph.push(lines[i++]);html+='<p>'+paragraph.map(inline).join('<br>')+'</p>';
 }
 return html;
}

export function markdownEditor(input,title) {
 const wrapper=document.createElement('div');wrapper.className='markdown-editor';
 const tabs=document.createElement('div');tabs.className='markdown-tabs';tabs.setAttribute('aria-label',title+'编辑模式');
 const toolbar=document.createElement('div');toolbar.className='markdown-toolbar';
 const preview=document.createElement('div');preview.className='markdown-preview';preview.setAttribute('aria-label',title+'预览');
 const area=document.createElement('div');area.className='markdown-area';area.append(input,preview);
 input.setAttribute('aria-label',title+' Markdown 源码');input.spellcheck=false;
 const makeButton=(label,action)=>{const b=document.createElement('button');b.type='button';b.textContent=label;b.onclick=action;return b;};
 const refresh=()=>{preview.innerHTML=renderMarkdown(input.value)||'<p class="markdown-empty">暂无内容，切换到源码开始编写。</p>';};
 const setMode=mode=>{wrapper.dataset.mode=mode;input.hidden=mode==='preview';preview.hidden=mode==='source';toolbar.hidden=mode==='preview';for(const b of tabs.children)b.setAttribute('aria-pressed',String(b.dataset.mode===mode));refresh();};
 for(const [mode,label] of [['source','源码'],['preview','预览'],['split','分栏']]){const b=makeButton(label,()=>setMode(mode));b.dataset.mode=mode;tabs.append(b);}
 const insert=(before,after='',fallback='文字')=>{const start=input.selectionStart,end=input.selectionEnd;const selected=input.value.slice(start,end)||fallback;input.setRangeText(before+selected+after,start,end,'select');input.focus();input.dispatchEvent(new Event('input',{bubbles:true}));};
 for(const [label,before,after,fallback] of [['标题','## ','','标题'],['粗体','**','**','文字'],['斜体','*','*','文字'],['列表','\n- ','','列表项'],['引用','\n> ','','引用内容'],['代码','\n```\n','\n```\n','代码'],['链接','[','](https://example.com)','链接文字']])toolbar.append(makeButton(label,()=>insert(before,after,fallback)));
 input.addEventListener('input',refresh);
 // Keep native required-field validation focusable even while previewing.
 input.addEventListener('invalid',()=>setMode('source'));
 wrapper.append(tabs,toolbar,area);setMode('source');return wrapper;
}
