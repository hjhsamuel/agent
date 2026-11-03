export function modelBody(values) {
 const data={...values};
 if (typeof data.extra==='string' && data.extra.trim()) {
  try { data.extra=JSON.parse(data.extra); } catch { throw Error('额外配置必须是有效的 JSON 对象。'); }
  if (!data.extra || typeof data.extra!=='object' || Array.isArray(data.extra)) throw Error('额外配置必须是 JSON 对象，不能是数组、null 或基本类型。');
 } else { delete data.extra; }
 return data;
}
export function modelValues(model,detail) {
 return {...model,context_limit:detail.context_limit,extra:JSON.stringify(detail.extra??{},null,2)};
}
