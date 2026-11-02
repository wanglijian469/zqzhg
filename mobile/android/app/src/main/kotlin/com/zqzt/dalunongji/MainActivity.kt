package com.zqzt.dalunongji

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import android.app.Activity
import android.content.Intent
import android.content.ClipData
import androidx.core.content.FileProvider
import java.io.File

class MainActivity : FlutterActivity() {
 private var officialVideos: OfficialVideoFactory? = null
 private var videoResult: MethodChannel.Result? = null
 private var workspaceResult: MethodChannel.Result? = null
 private var saveResult: MethodChannel.Result? = null
 private var saveBytes: ByteArray? = null
 override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
  super.configureFlutterEngine(flutterEngine)
  officialVideos = OfficialVideoFactory(this, flutterEngine.dartExecutor.binaryMessenger).also {
   flutterEngine.platformViewsController.registry.registerViewFactory("zqzg/official-video", it)
  }
  MethodChannel(flutterEngine.dartExecutor.binaryMessenger,"com.zqzt.dalunongji/video").setMethodCallHandler { call,result ->
   if(call.method!="open"){result.notImplemented()}else if(videoResult!=null){result.error("busy","A video is already open",null)}else{try{videoResult=result;startActivityForResult(Intent(this,VideoPlayerActivity::class.java).putExtra("url",call.argument<String>("url")).putExtra("width",call.argument<Number>("width")?.toInt()?:0).putExtra("height",call.argument<Number>("height")?.toInt()?:0),4023)}catch(_:Exception){videoResult=null;result.error("video_failed","Cannot open player",null)}}
  }
  MethodChannel(flutterEngine.dartExecutor.binaryMessenger,"com.zqzt.dalunongji/workspace").setMethodCallHandler { call,result ->
   if(call.method!="open") { result.notImplemented();return@setMethodCallHandler }
   if(workspaceResult!=null) { result.error("busy","Workspace is already open",null);return@setMethodCallHandler }
   try {
    val args=call.arguments as? Map<*,*> ?: error("Missing workspace options")
    val payload=org.json.JSONObject(args).toString()
    require(payload.length<65536)
    workspaceResult=result
    startActivityForResult(Intent(this,WorkspaceActivity::class.java).putExtra("payload",payload),4022)
   } catch(_:Exception) { workspaceResult=null;result.error("workspace_failed","Cannot open workspace",null) }
  }
  MethodChannel(flutterEngine.dartExecutor.binaryMessenger,"com.zqzt.dalunongji/sharing").setMethodCallHandler { call,result ->
   try {
    when(call.method) {
     "shareText" -> {
      val intent=Intent(Intent.ACTION_SEND).apply { type="text/plain";putExtra(Intent.EXTRA_TEXT,call.argument<String>("text"));putExtra(Intent.EXTRA_SUBJECT,call.argument<String>("title")) }
      startActivity(Intent.createChooser(intent,null));result.success(true)
     }
     "shareImage" -> {
      val bytes=call.argument<ByteArray>("bytes") ?: error("Missing image")
      val dir=File(cacheDir,"share").apply{mkdirs()}
      dir.listFiles()?.filter{System.currentTimeMillis()-it.lastModified()>86400000}?.forEach{it.delete()}
      val image=File(dir,"showroom-${System.currentTimeMillis()}.png");image.writeBytes(bytes)
      val uri=FileProvider.getUriForFile(this,"$packageName.sharefiles",image)
      val intent=Intent(Intent.ACTION_SEND).apply {type="image/png";putExtra(Intent.EXTRA_STREAM,uri);putExtra(Intent.EXTRA_SUBJECT,call.argument<String>("title"));clipData=ClipData.newRawUri("showroom",uri);addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)}
      startActivity(Intent.createChooser(intent,null));result.success(true)
     }
     "saveImage", "saveFile" -> {
      if(saveResult!=null){result.error("busy","A save is in progress",null)}else{
       saveBytes=call.argument<ByteArray>("bytes") ?: error("Missing image");saveResult=result
       val intent=Intent(Intent.ACTION_CREATE_DOCUMENT).apply{addCategory(Intent.CATEGORY_OPENABLE);type=call.argument<String>("mime") ?: "image/png";putExtra(Intent.EXTRA_TITLE,call.argument<String>("name") ?: "showroom.png")}
       startActivityForResult(intent,4021)
      }
     }
     else -> result.notImplemented()
    }
   }catch(e:Exception){if((call.method=="saveImage" || call.method=="saveFile")){saveResult=null;saveBytes=null};result.error("share_failed",e.message,null)}
  }
 }
 override fun onActivityResult(requestCode:Int,resultCode:Int,data:Intent?){
  super.onActivityResult(requestCode,resultCode,data)
  if(requestCode==4023){videoResult?.success(null);videoResult=null;return}
  if(requestCode==4022){val result=workspaceResult;workspaceResult=null;if(resultCode==Activity.RESULT_OK&&data!=null)result?.success(mapOf("token" to data.getStringExtra("token"),"language" to data.getStringExtra("language")))else result?.success(null);return}
  if(requestCode!=4021)return
  val result=saveResult;val bytes=saveBytes;saveResult=null;saveBytes=null
  if(resultCode!=Activity.RESULT_OK||data?.data==null){result?.success(false);return}
  try{val stream=contentResolver.openOutputStream(data.data!!,"w") ?: error("Cannot save image");stream.use{it.write(bytes ?: error("Missing image"))};result?.success(true)}catch(e:Exception){result?.error("save_failed",e.message,null)}
 }
 @Deprecated("Android back compatibility") override fun onBackPressed(){if(officialVideos?.exitFullscreen()!=true)super.onBackPressed()}
 override fun onPause(){officialVideos?.pauseAll();super.onPause()}
 override fun onDestroy(){officialVideos?.disposeAll();officialVideos=null;videoResult?.success(null);videoResult=null;workspaceResult?.success(null);workspaceResult=null;saveResult?.success(false);saveResult=null;saveBytes=null;super.onDestroy()}
}
