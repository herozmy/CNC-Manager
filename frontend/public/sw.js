// 仅在页面导航无法连接服务器时提供提示，业务接口沿用浏览器的网络请求。
self.addEventListener('fetch', (event) => {
  const request = event.request
  const url = new URL(request.url)
  if (request.method !== 'GET' || request.mode !== 'navigate' ||
      url.origin !== self.location.origin || url.pathname.startsWith('/api/')) return

  event.respondWith(fetch(request).catch(() => new Response(
    '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>CNC 台账</title><main style="margin:12vh 8vw;font:1rem system-ui;color:#0f172a"><h1>无法连接 CNC 台账</h1><p>请检查网络和服务器，连接恢复后重试。</p><a href="">重新连接</a></main></html>',
    { status: 503, headers: { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store' } },
  )))
})
