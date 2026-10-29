import http from 'node:http';
import https from 'node:https';
import { readFile } from 'node:fs/promises';
let port;
try {
  const config = JSON.parse(await readFile(new URL('./config.json', import.meta.url), 'utf8'));
  port = config.port;
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error('port 必须是 1 到 65535 之间的整数');
  }
} catch (error) {
  console.error('无法加载 ui/config.json：'+error.message);
  process.exit(1);
}
const target = new URL(process.env.CAGENT_API_URL || 'http://127.0.0.1:8080');
const files = {'/':['index.html','text/html'], '/app.js':['app.js','text/javascript'], '/admin.js':['admin.js','text/javascript'], '/markdown.js':['markdown.js','text/javascript'], '/style.css':['style.css','text/css']};
http.createServer(async (req,res) => {
  if (req.url.startsWith('/cagent/v1/')) {
    const upstream = (target.protocol === 'https:' ? https : http).request(new URL(req.url,target), {method:req.method, headers:{...req.headers, host:target.host}}, response => {
      res.writeHead(response.statusCode,response.headers); response.pipe(res);
    });
    upstream.on('error',()=> {if (!res.headersSent) res.writeHead(502,{'Content-Type':'application/json'}); res.end(JSON.stringify({code:502,message:'无法连接后端，请检查 CAGENT_API_URL 和后端服务。'}));});
    res.on('close',()=>upstream.destroy()); req.pipe(upstream); return;
  }
  const file = files[req.url.split('?')[0]];
  if (!file) {res.writeHead(404);res.end();return;}
  try {res.writeHead(200,{'Content-Type':file[1]+'; charset=utf-8','Cache-Control':'no-store'});res.end(await readFile(new URL('./dist/'+file[0],import.meta.url)));}
  catch {res.end('文件读取失败');}
}).listen(port,'0.0.0.0',()=>console.log('CAgent UI: http://127.0.0.1:'+port));
