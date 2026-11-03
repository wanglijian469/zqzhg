import com.zqzt.dalunongji.OfficialVideoDocument;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;

/** Browser QA invokes the production document and request router, not a copied allowlist. */
public final class OfficialVideoDocumentProbe {
 public static void main(String[] args)throws Exception {
  boolean wide=args.length==3&&args[0].equals("render")&&Boolean.parseBoolean(args[2]);
  String videoID=wide?"7690195495465012480":"7136314708138724640";
  OfficialVideoDocument document=new OfficialVideoDocument(
   "https://open.douyin.com/player/video?vid="+videoID+"&autoplay=0",wide,
   "c90aa3a0-f4c6-4a41-8b09-00891214789b");
  if((args.length==2||args.length==3)&&args[0].equals("render")){
   Files.write(Paths.get(args[1]),document.html.getBytes(StandardCharsets.UTF_8));System.out.println(document.url);
  }else if(args.length==3&&args[0].equals("route")){
   System.out.println(document.route(args[2],Boolean.parseBoolean(args[1])));
  }else throw new IllegalArgumentException("render path | route mainFrame URL");
 }
}
