package com.zqzt.dalunongji;
import java.net.URI;
import java.net.URLDecoder;
import java.util.*;
import java.util.regex.Pattern;
/** Shared, JVM-testable policy for the token-free official player window. */
public final class OfficialVideoPolicy {
 private OfficialVideoPolicy() {}
 private static final Pattern ID=Pattern.compile("[1-9][0-9]{8,23}");
 private static final List<String> ROOTS=Arrays.asList("douyin.com","douyincdn.com","douyinpic.com","douyinvod.com","byteimg.com","ibyteimg.com","pstatp.com","bytedance.com","bytedance.net","snssdk.com","amemv.com","bytegecko.com","bytegoofy.com");
 private static URI https(String raw) throws Exception {
  URI u=new URI(raw);
  if(!"https".equals(u.getScheme())||u.getHost()==null||u.getPort()!=-1||u.getRawUserInfo()!=null)throw new IllegalArgumentException();
  return u;
 }
 public static boolean trustedPlayer(String raw) {
  try {
   URI u=https(raw);if(!"open.douyin.com".equalsIgnoreCase(u.getHost())||!"/player/video".equals(u.getPath())||u.getRawFragment()!=null)return false;
   String[] query=u.getRawQuery().split("&",-1);if(query.length!=2)return false;Map<String,String> values=new HashMap<>();
   for(String pair:query){String[] kv=pair.split("=",2);if(kv.length!=2)return false;String key=URLDecoder.decode(kv[0],"UTF-8");if(values.put(key,URLDecoder.decode(kv[1],"UTF-8"))!=null)return false;}
   return values.size()==2&&"0".equals(values.get("autoplay"))&&ID.matcher(values.get("vid")==null?"":values.get("vid")).matches();
  }catch(Exception ignored){return false;}
 }
 public static boolean allowedResource(String raw) {
  try {
   URI u=https(raw);String host=u.getHost().toLowerCase(Locale.ROOT);
   for(String root:ROOTS)if(host.equals(root)||host.endsWith("."+root))return true;
   // Exact hosts and namespaces observed in the official player's own page.
   // open-douyin.com is distinct from douyin.com; the old suffix list missed it.
   if(host.equals("lf-developer-cdn.open-douyin.com"))return u.getPath().startsWith("/obj/open-platform-scm/");
   if(host.equals("lf-c-flwb.bytetos.com"))return u.getPath().startsWith("/obj/rc-client-security/");
   return false;
  }catch(Exception ignored){return false;}
 }
 public static boolean criticalResource(String raw) {
  try {
   URI u=https(raw);String host=u.getHost().toLowerCase(Locale.ROOT),path=u.getPath();
   return host.equals("open.douyin.com")&&(path.equals("/player/video")||path.startsWith("/api/douyin/v1/open/auth/embed_video"))
    ||host.equals("lf-developer-cdn.open-douyin.com")&&path.startsWith("/obj/open-platform-scm/")&&path.endsWith(".js")
    ||host.equals("lf-c-flwb.bytetos.com")&&path.startsWith("/obj/rc-client-security/")&&path.endsWith(".js");
  }catch(Exception ignored){return false;}
 }
 /** Certificate failures are always rejected, but unrelated resources must not tear down playback. */
 public static boolean failureIsFatal(String kind, boolean mainFrame, String raw) {
  if("slow_load".equals(kind))return false;
  return mainFrame||criticalResource(raw);
 }
 /** Only an origin host may leave the native networking layer; never cookies, queries or signed paths. */
 public static String diagnosticHost(String raw) {
  try{return https(raw).getHost().toLowerCase(Locale.ROOT);}catch(Exception ignored){return "";}
 }
 public static String diagnosticScheme(String raw) {
  if(raw==null)return "";int end=raw.indexOf(':');
  if(end<1||end>20)return "";String scheme=raw.substring(0,end).toLowerCase(Locale.ROOT);
  return scheme.matches("[a-z][a-z0-9+.-]*")?scheme:"";
 }
 public static String originalVideoURL(String raw) {
  if(!trustedPlayer(raw))return "";
  try{for(String pair:new URI(raw).getRawQuery().split("&")){String[] kv=pair.split("=",2);if("vid".equals(URLDecoder.decode(kv[0],"UTF-8")))return "https://www.douyin.com/video/"+URLDecoder.decode(kv[1],"UTF-8");}}catch(Exception ignored){}
  return "";
 }
}
