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
  System.out.println(checks+" official player policy checks passed");
 }
}
