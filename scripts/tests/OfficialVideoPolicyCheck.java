package com.zqzt.dalunongji;
public final class OfficialVideoPolicyCheck {
 private static int checks;
 private static void check(boolean value){checks++;if(!value)throw new AssertionError("Policy check "+checks);}
 public static void main(String[] args){
  String player="https://open.douyin.com/player/video?vid=7136314708138724640&autoplay=0";
  check(OfficialVideoPolicy.trustedPlayer(player));check(OfficialVideoPolicy.originalVideoURL(player).equals("https://www.douyin.com/video/7136314708138724640"));
  for(String bad:new String[]{player.replace("https:","http:"),player.replace("open.douyin.com","open.douyin.com.evil.test"),player.replace("open.douyin.com","x@open.douyin.com"),player+"#x",player+"&vid=7136314708138724640",player.replace("autoplay=0","autoplay=1"),player.replace("7136314708138724640","bad"),player.replace("open.douyin.com","open.douyin.com:8443")})check(!OfficialVideoPolicy.trustedPlayer(bad));
  String cdn="https://lf-developer-cdn.open-douyin.com/obj/open-platform-scm/open_douyin_sdk_demo/static/js/video.0770dd9d.js";
  check(OfficialVideoPolicy.allowedResource(cdn));check(OfficialVideoPolicy.criticalResource(cdn));
  check(OfficialVideoPolicy.allowedResource("https://lf-c-flwb.bytetos.com/obj/rc-client-security/c-webmssdk/1.0.0.22/webmssdk.js"));
  for(String bad:new String[]{cdn.replace("https:","http:"),cdn.replace("open-douyin.com","open-douyin.com.evil.test"),cdn.replace("/obj/open-platform-scm/","/arbitrary/"),"https://unknown.bytetos.com/script.js","file:///etc/passwd","javascript:alert(1)","https://127.0.0.1/a.js","https://mcs.zijieapi.com/webid"})check(!OfficialVideoPolicy.allowedResource(bad));
  check(OfficialVideoPolicy.allowedResource("https://v26-web.douyinvod.com/video/test"));check(OfficialVideoPolicy.criticalResource("https://open.douyin.com/api/douyin/v1/open/auth/embed_video/"));
  check(!OfficialVideoPolicy.failureIsFatal("slow_load",true,player));
  check(!OfficialVideoPolicy.failureIsFatal("tls",false,"https://mon.zijieapi.com/monitor"));
  check(!OfficialVideoPolicy.failureIsFatal("tls",false,"https://p3-pc.douyinpic.com/image"));
  check(OfficialVideoPolicy.failureIsFatal("tls",false,player));
  check(OfficialVideoPolicy.failureIsFatal("tls",false,cdn));
  check(OfficialVideoPolicy.failureIsFatal("network",true,"https://zqzhg.com/"));
  check(OfficialVideoPolicy.diagnosticHost("https://open.douyin.com/player/video?token=SECRET#x").equals("open.douyin.com"));
  check(OfficialVideoPolicy.diagnosticHost("https://user:SECRET@open.douyin.com/private").isEmpty());
  check(OfficialVideoPolicy.diagnosticHost("file:///private/SECRET").isEmpty());
  OfficialVideoLoadState state=new OfficialVideoLoadState();int old=state.begin();
  check(state.slow(old));check(state.current(old));check(!state.slow(old));
  check(state.ready(old));check(!state.ready(old));check(!state.slow(old));
  int next=state.begin();check(!state.ready(old));check(!state.slow(old));check(state.current(next));
  state.stop();check(!state.current(next));check(!state.ready(next));check(!state.slow(next));
  OfficialVideoDocument doc=new OfficialVideoDocument(player,false,"c90aa3a0-f4c6-4a41-8b09-00891214789b");
  check(!OfficialVideoPolicy.allowedResource(doc.url)); // This is exactly why the old remote-only filter rejected an owned page.
  check(doc.route(doc.url,true)==OfficialVideoDocument.DOCUMENT);
  check(doc.route(doc.url,false)==OfficialVideoDocument.BLOCK);
  check(doc.route(player,false)==OfficialVideoDocument.NETWORK);
  check(doc.route(player,true)==OfficialVideoDocument.BLOCK);
  check(doc.route(cdn,false)==OfficialVideoDocument.NETWORK);
  for(String bad:new String[]{doc.url+"?x=1",doc.url+"#x",doc.url.replace("zqzhg.com","evil.test"),doc.url.replace("c90aa3a0","d90aa3a0"),"https://zqzhg.com/","https://zqzhg.com/merchant","data:text/html,<script>SECRET</script>","about:blank","file:///private/file","https://unknown.test/script.js"}){
   check(doc.route(bad,true)==OfficialVideoDocument.BLOCK);check(doc.route(bad,false)==OfficialVideoDocument.BLOCK);
  }
  OfficialVideoDocument replacement=new OfficialVideoDocument(player,true,"d90aa3a0-f4c6-4a41-8b09-00891214789b");
  check(replacement.route(doc.url,true)==OfficialVideoDocument.BLOCK);
  check(doc.html.contains("sandbox=\"allow-scripts allow-same-origin allow-presentation\""));
  check(doc.html.contains("&amp;autoplay=0"));check(doc.html.contains("const w=324,h=672"));
  check(replacement.html.contains("const w=800,h=485"));
  boolean refused=false;try{new OfficialVideoDocument(player,false,"\"><script>SECRET</script>");}catch(IllegalArgumentException expected){refused=true;}check(refused);
  refused=false;try{new OfficialVideoDocument("https://evil.test/",false,"c90aa3a0-f4c6-4a41-8b09-00891214789b");}catch(IllegalArgumentException expected){refused=true;}check(refused);
  check(OfficialVideoPolicy.diagnosticScheme("data:text/html,SECRET").equals("data"));
  check(OfficialVideoPolicy.diagnosticScheme("https://open.douyin.com/?token=SECRET").equals("https"));
  check(OfficialVideoPolicy.diagnosticScheme("not a scheme:SECRET").isEmpty());
  System.out.println(checks+" official player policy checks passed");
 }
}
