package com.zqzt.dalunongji;

/** App-owned HTML has its own exact local route; it is never a remote allowlist exception. */
public final class OfficialVideoDocument {
 public static final int DOCUMENT=1, NETWORK=2, BLOCK=3;
 public final String url;
 public final String html;
 public OfficialVideoDocument(String playerURL, boolean wide, String nonce) {
  if(!OfficialVideoPolicy.trustedPlayer(playerURL)||nonce==null||!nonce.matches("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"))throw new IllegalArgumentException("Invalid official player document");
  url="https://zqzhg.com/__app_official_video/"+nonce+"/index.html";
  String safe=playerURL.replace("&","&amp;").replace("\"","&quot;").replace("<","&lt;").replace(">","&gt;");
  html="""
<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-__NONCE__'; frame-src https://open.douyin.com; object-src 'none'; base-uri 'none'; form-action 'none'"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden;background:#000}#stage{position:fixed;top:0;left:0;right:0;bottom:0;overflow:hidden;contain:layout size}#box{position:absolute;left:50%;top:50%;overflow:hidden;transform:translate(-50%,-50%)}iframe{display:block;border:0;max-width:none;transform-origin:0 0}iframe:fullscreen{transform:none!important;width:100vw!important;height:100vh!important}</style></head><body><div id="stage"><div id="box"><iframe id="f" width="__W__" height="__H__" title="抖音官方播放器" src="__PLAYER__" sandbox="allow-scripts allow-same-origin allow-presentation" referrerpolicy="origin" allow="fullscreen; encrypted-media" allowfullscreen></iframe></div></div><script nonce="__NONCE__">const w=__W__,h=__H__,v=document.getElementById('stage'),b=document.getElementById('box'),f=document.getElementById('f');function fit(){const s=Math.min(1,v.clientWidth/w,v.clientHeight/h);b.style.width=w*s+'px';b.style.height=h*s+'px';f.style.width=w+'px';f.style.height=h+'px';f.style.transform='scale('+s+')'}fit();addEventListener('resize',fit);if(window.ResizeObserver)new ResizeObserver(fit).observe(v);</script></body></html>
""".replace("__NONCE__",nonce).replace("__PLAYER__",safe)
    .replace("__W__",wide?"800":"324").replace("__H__",wide?"485":"672");
 }
 public int route(String requestURL, boolean mainFrame) {
  if(mainFrame&&url.equals(requestURL))return DOCUMENT;
  if(!mainFrame&&OfficialVideoPolicy.allowedResource(requestURL))return NETWORK;
  return BLOCK;
 }
}
