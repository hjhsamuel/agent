import test from 'node:test';
import assert from 'node:assert/strict';
import {renderMarkdown} from './dist/markdown.js';
test('renders headings, lists, quotes and fenced code',()=>{
 const html=renderMarkdown('# 标题\n\n- **重点**\n- *说明*\n\n> 引用\n\n```js\n<script>alert(1)</script>\n```');
 assert.match(html,/<h1>标题<\/h1>/);assert.match(html,/<ul><li><strong>重点<\/strong>/);assert.match(html,/<blockquote>/);assert.match(html,/<pre><code>&lt;script&gt;/);
});
test('escapes HTML and blocks active URL schemes',()=>{
 const html=renderMarkdown('<img src=x onerror=alert(1)>\n[x](javascript:alert)\n[x](data:text/html,evil)');
 assert.ok(!html.includes('<img'));assert.ok(!html.includes('<a'));
 assert.match(renderMarkdown('[正常](https://example.com?a=1&b=2)'),/href="https:\/\/example.com\?a=1&amp;b=2"/);
});
