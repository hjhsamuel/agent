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

test('renders tables, nested lists, tasks and underscore emphasis',()=>{
 const html=renderMarkdown('| 名称 | 值 |\n| --- | ---: |\n| __重点__ | _说明_ |\n\n- 父项\n  - 子项\n- [x] 完成');
 assert.match(html,/<table>/);assert.match(html,/<strong>重点<\/strong>/);assert.match(html,/<em>说明<\/em>/);
 assert.match(html,/<li>父项<ul><li>子项<\/li><\/ul><\/li>/);
 assert.match(html,/<input type="checkbox" disabled checked>/);
});
test('preserves code literals and renders incomplete streaming fences safely',()=>{
 assert.equal(renderMarkdown('`__literal__`'),'<p><code>__literal__</code></p>');
 assert.equal(renderMarkdown('~~~js\n<script>'),'<pre><code>&lt;script&gt;</code></pre>');
 assert.equal(renderMarkdown('~~~\ncode\n~~~\n\n标题\n==='),'<pre><code>code</code></pre><h1>标题</h1>');
});
