package com.zqzt.dalunongji

import android.app.Activity
import android.os.Bundle
import android.net.Uri
import android.view.View
import android.webkit.*
import android.widget.*
import android.graphics.Color

/** Token-free, single official-player window. No JS bridge, file access or arbitrary HTML. */
class VideoPlayerActivity : Activity() {
 private var web: WebView? = null
 private lateinit var frame: FrameLayout
 private var custom: View? = null
 private var fullscreenCallback: WebChromeClient.CustomViewCallback? = null
 override fun onCreate(state: Bundle?) {
  super.onCreate(state)
  val raw=intent.getStringExtra("url") ?: ""
  val uri=Uri.parse(raw)
  val id=uri.getQueryParameter("vid") ?: ""
  if(uri.scheme!="https" || uri.host!="open.douyin.com" || uri.port!=-1 || uri.userInfo!=null || uri.path!="/player/video" || !id.matches(Regex("[1-9][0-9]{8,23}")) || uri.getQueryParameter("autoplay")!="0" || uri.fragment!=null){finish();return}
  val wide=intent.getIntExtra("width",1920)>=intent.getIntExtra("height",1080)
  val logicalWidth=if(wide)800 else 324
  val logicalHeight=if(wide)485 else 672
  val nonce=java.util.UUID.randomUUID().toString()
  val safe=raw.replace("&","&amp;").replace("\"","&quot;").replace("<","&lt;").replace(">","&gt;")
  frame=FrameLayout(this).apply{setBackgroundColor(Color.BLACK)}
  val layout=LinearLayout(this).apply{orientation=LinearLayout.VERTICAL}
  layout.addView(Button(this).apply{text="关闭视频";setOnClickListener{finish()}})
  val player=WebView(this);web=player
  player.setBackgroundColor(Color.BLACK)
  player.settings.apply{javaScriptEnabled=true;domStorageEnabled=true;allowFileAccess=false;allowContentAccess=false;mediaPlaybackRequiresUserGesture=true;setSupportMultipleWindows(false);mixedContentMode=WebSettings.MIXED_CONTENT_NEVER_ALLOW}
  player.webViewClient=object:WebViewClient(){
   override fun shouldOverrideUrlLoading(view:WebView,request:WebResourceRequest):Boolean = request.isForMainFrame || request.url.scheme!="https" || request.url.host!="open.douyin.com"
   override fun onReceivedSslError(view:WebView,handler:SslErrorHandler,error:android.net.http.SslError){handler.cancel()}
  }
  player.webChromeClient=object:WebChromeClient(){
   override fun onShowCustomView(view:View,callback:CustomViewCallback){if(custom!=null){callback.onCustomViewHidden();return};custom=view;fullscreenCallback=callback;frame.addView(view,FrameLayout.LayoutParams(-1,-1));window.decorView.systemUiVisibility=5894}
   override fun onHideCustomView(){exitFullscreen()}
  }
  layout.addView(player,LinearLayout.LayoutParams(-1,0,1f));frame.addView(layout,FrameLayout.LayoutParams(-1,-1));setContentView(frame)
  player.loadDataWithBaseURL("https://zqzhg.com/", """<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-$nonce'; frame-src https://open.douyin.com; object-src 'none'; base-uri 'none'"><style>html,body{margin:0;height:100%;background:#000}#box{position:absolute;left:50%;top:50%;transform:translate(-50%,-50%)}iframe{border:0;transform-origin:0 0}iframe:fullscreen{transform:none!important;width:100vw!important;height:100vh!important}</style></head><body><div id="box"><iframe id="f" width="$logicalWidth" height="$logicalHeight" title="抖音视频" src="$safe" sandbox="allow-scripts allow-same-origin allow-presentation" referrerpolicy="origin" allow="fullscreen; encrypted-media; picture-in-picture" allowfullscreen></iframe></div><script nonce="$nonce">function fit(){const w=$logicalWidth,h=$logicalHeight,s=Math.min(1,innerWidth/w,innerHeight/h),b=document.getElementById('box'),f=document.getElementById('f');b.style.width=w*s+'px';b.style.height=h*s+'px';f.style.width=w+'px';f.style.height=h+'px';f.style.transform='scale('+s+')'}fit();addEventListener('resize',fit);</script></body></html>""", "text/html","UTF-8",null)
 }
 private fun exitFullscreen(){val callback=fullscreenCallback;fullscreenCallback=null;custom?.let{frame.removeView(it)};custom=null;callback?.onCustomViewHidden();window.decorView.systemUiVisibility=0}
 @Deprecated("Android back compatibility") override fun onBackPressed(){if(custom!=null)exitFullscreen() else finish()}
 private fun release(){exitFullscreen();web?.apply{stopLoading();loadUrl("about:blank");onPause();removeAllViews();destroy()};web=null}
 override fun onPause(){release();if(!isFinishing)finish();super.onPause()}
 override fun onDestroy(){release();super.onDestroy()}
}
