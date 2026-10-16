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
 private var saveResult: MethodChannel.Result? = null
 private var saveBytes: ByteArray? = null
 override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
  super.configureFlutterEngine(flutterEngine)
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
  if(requestCode!=4021)return
  val result=saveResult;val bytes=saveBytes;saveResult=null;saveBytes=null
  if(resultCode!=Activity.RESULT_OK||data?.data==null){result?.success(false);return}
  try{val stream=contentResolver.openOutputStream(data.data!!,"w") ?: error("Cannot save image");stream.use{it.write(bytes ?: error("Missing image"))};result?.success(true)}catch(e:Exception){result?.error("save_failed",e.message,null)}
 }
 override fun onDestroy(){saveResult?.success(false);saveResult=null;saveBytes=null;super.onDestroy()}
}
