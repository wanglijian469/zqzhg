package com.zqzt.dalunongji

import android.app.Activity
import android.content.Context
import android.graphics.Color
import android.net.Uri
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.view.Gravity
import android.view.MotionEvent
import android.view.View
import android.view.ViewGroup
import android.webkit.*
import android.widget.Button
import android.widget.FrameLayout
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.StandardMessageCodec
import io.flutter.plugin.platform.PlatformView
import io.flutter.plugin.platform.PlatformViewFactory
import java.io.ByteArrayInputStream
import java.util.UUID

/** Owns only in-page platform views. It never launches an Activity or external player. */
class OfficialVideoFactory(
    private val activity: Activity,
    private val messenger: BinaryMessenger,
) : PlatformViewFactory(StandardMessageCodec.INSTANCE) {
    private val players = linkedSetOf<OfficialVideoView>()

    override fun create(context: Context, viewId: Int, args: Any?): PlatformView {
        players.toList().forEach { it.stop(notify = true) }
        return OfficialVideoView(activity, messenger, viewId, args as? Map<*, *>) {
            players.remove(it)
        }.also { players.add(it) }
    }

    fun exitFullscreen(): Boolean = players.toList().any { it.exitFullscreen() }
    fun pauseAll() = players.toList().forEach { it.stop(notify = true) }
    fun disposeAll() = players.toList().forEach { it.dispose() }
}

/** Only a validated public official URL and dimensions enter this component. No JS bridge. */
class OfficialVideoView(
    private val activity: Activity,
    messenger: BinaryMessenger?,
    viewId: Int,
    args: Map<*, *>?,
    private val onEvent: ((String, Any?) -> Unit)? = null,
    private var onDisposed: ((OfficialVideoView) -> Unit)?,
) : PlatformView {
    private val frame = FrameLayout(activity).apply { setBackgroundColor(Color.BLACK) }
    private val channel = messenger?.let { MethodChannel(it, "zqzg/official-video/$viewId") }
    private val handler = Handler(Looper.getMainLooper())
    private val rawUrl = args?.get("url") as? String ?: ""
    private val wide = dimension(args?.get("width")) >= dimension(args?.get("height"))
    private var web: WebView? = null
    private var disposed = false
    private var failed = false
    private var overlay: FrameLayout? = null
    private var custom: View? = null
    private var customCallback: WebChromeClient.CustomViewCallback? = null
    private var priorSystemUi = 0
    private var lastUserTouch = Long.MIN_VALUE

    init {
        channel?.setMethodCallHandler { call, result ->
            when (call.method) {
                "stop" -> { stop(); result.success(null) }
                "start", "retry" -> { if (!disposed) load(); result.success(null) }
                "exitFullscreen" -> result.success(exitFullscreen())
                else -> result.notImplemented()
            }
        }
        // Flutter sends start only after installing this view's event handler.
    }

    override fun getView(): View = frame

    private fun dimension(value: Any?): Int = (value as? Number)?.toInt()?.coerceIn(0, 16384) ?: 0

    private fun trustedPlayer(raw: String) = OfficialVideoPolicy.trustedPlayer(raw)
    private fun allowedResource(uri: Uri) = OfficialVideoPolicy.allowedResource(uri.toString())

    private fun event(name: String, value: Any? = null) {
        if (!disposed) {
            channel?.invokeMethod(name, value)
            onEvent?.invoke(name, value)
        }
    }

    private fun fail() {
        if (disposed || failed) return
        failed = true
        releaseWeb()
        event("error")
    }

    private fun isPlayerRequest(request: WebResourceRequest): Boolean =
        request.isForMainFrame || OfficialVideoPolicy.criticalResource(request.url.toString())

    private fun load() {
        handler.removeCallbacksAndMessages(null)
        releaseWeb()
        failed = false
        lastUserTouch = Long.MIN_VALUE
        if (!trustedPlayer(rawUrl)) { fail(); return }
        val player = WebView(activity)
        web = player
        val loadTimeout = Runnable { if (web === player && !disposed) fail() }
        handler.postDelayed(loadTimeout, 20000)
        player.setBackgroundColor(Color.BLACK)
        player.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            allowFileAccess = false
            allowContentAccess = false
            mediaPlaybackRequiresUserGesture = true
            javaScriptCanOpenWindowsAutomatically = false
            setSupportMultipleWindows(false)
            mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
            setGeolocationEnabled(false)
        }
        player.setOnTouchListener { _, motion ->
            if (motion.action == MotionEvent.ACTION_UP) lastUserTouch = SystemClock.uptimeMillis()
            false
        }
        player.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean =
                request.isForMainFrame || !allowedResource(request.url)

            override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
                if (allowedResource(request.url)) return null
                // The only app-owned document is supplied directly by loadDataWithBaseURL.
                return WebResourceResponse("text/plain", "UTF-8", 403, "Blocked by player policy", mapOf("Cache-Control" to "no-store"), ByteArrayInputStream(ByteArray(0)))
            }

            override fun onPageFinished(view: WebView, url: String) {
                if (web === view && !failed) {
                    handler.removeCallbacks(loadTimeout)
                    event("loaded")
                }
            }

            override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                if (isPlayerRequest(request)) handler.post { if (web === view) fail() }
            }

            override fun onReceivedHttpError(view: WebView, request: WebResourceRequest, response: WebResourceResponse) {
                if (isPlayerRequest(request) && response.statusCode >= 400) handler.post { if (web === view) fail() }
            }

            override fun onReceivedSslError(view: WebView, ssl: SslErrorHandler, error: android.net.http.SslError) {
                ssl.cancel()
                handler.post { if (web === view) fail() }
            }

            override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
                handler.post { if (web === view) fail() }
                return true
            }
        }
        player.webChromeClient = object : WebChromeClient() {
            override fun onShowCustomView(view: View, callback: CustomViewCallback) {
                // Fullscreen is accepted only immediately after an explicit touch in this player.
                if (disposed || web !== player || custom != null || SystemClock.uptimeMillis() - lastUserTouch !in 0..2500) {
                    callback.onCustomViewHidden()
                    return
                }
                val decor = activity.window.decorView as? ViewGroup
                if (decor == null) { callback.onCustomViewHidden(); return }
                (view.parent as? ViewGroup)?.removeView(view)
                priorSystemUi = activity.window.decorView.systemUiVisibility
                val host = FrameLayout(activity).apply {
                    setBackgroundColor(Color.BLACK)
                    isClickable = true
                    isFocusableInTouchMode = true
                    addView(view, FrameLayout.LayoutParams(-1, -1))
                    addView(Button(activity).apply {
                        text = "退出全屏"
                        setOnClickListener { exitFullscreen() }
                    }, FrameLayout.LayoutParams(-2, -2, Gravity.TOP or Gravity.END))
                }
                custom = view
                customCallback = callback
                overlay = host
                decor.addView(host, ViewGroup.LayoutParams(-1, -1))
                host.requestFocus()
                activity.window.decorView.systemUiVisibility = 5894
                event("fullscreen", true)
            }

            override fun onHideCustomView() { if (web === player) exitFullscreen() }
            override fun onJsAlert(view: WebView?, url: String?, message: String?, result: JsResult): Boolean {
                result.cancel()
                return true
            }
            override fun onJsConfirm(view: WebView?, url: String?, message: String?, result: JsResult): Boolean {
                result.cancel()
                return true
            }
            override fun onJsPrompt(view: WebView?, url: String?, message: String?, defaultValue: String?, result: JsPromptResult): Boolean {
                result.cancel()
                return true
            }
            override fun onPermissionRequest(request: PermissionRequest) { request.deny() }
            override fun onGeolocationPermissionsShowPrompt(origin: String?, callback: GeolocationPermissions.Callback?) {
                callback?.invoke(origin, false, false)
            }
            override fun onShowFileChooser(view: WebView?, callback: ValueCallback<Array<Uri>>?, params: FileChooserParams?): Boolean {
                callback?.onReceiveValue(null)
                return true
            }
        }
        frame.addView(player, FrameLayout.LayoutParams(-1, -1))
        val logicalWidth = if (wide) 800 else 324
        val logicalHeight = if (wide) 485 else 672
        val nonce = UUID.randomUUID().toString()
        val safe = rawUrl.replace("&", "&amp;").replace("\"", "&quot;").replace("<", "&lt;").replace(">", "&gt;")
        player.loadDataWithBaseURL(
            "https://zqzhg.com/",
            """<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-$nonce'; frame-src https://open.douyin.com; object-src 'none'; base-uri 'none'; form-action 'none'"><style>html,body{margin:0;height:100%;overflow:hidden;background:#000}#box{position:absolute;left:50%;top:50%;transform:translate(-50%,-50%)}iframe{border:0;transform-origin:0 0}iframe:fullscreen{transform:none!important;width:100vw!important;height:100vh!important}</style></head><body><div id="box"><iframe id="f" width="$logicalWidth" height="$logicalHeight" title="抖音官方播放器" src="$safe" sandbox="allow-scripts allow-same-origin allow-presentation" referrerpolicy="origin" allow="fullscreen; encrypted-media" allowfullscreen></iframe></div><script nonce="$nonce">function fit(){const w=$logicalWidth,h=$logicalHeight,s=Math.min(1,innerWidth/w,innerHeight/h),b=document.getElementById('box'),f=document.getElementById('f');b.style.width=w*s+'px';b.style.height=h*s+'px';f.style.width=w+'px';f.style.height=h+'px';f.style.transform='scale('+s+')'}fit();addEventListener('resize',fit);</script></body></html>""",
            "text/html", "UTF-8", null,
        )
    }

    fun start() { if (!disposed) load() }

    fun exitFullscreen(): Boolean {
        if (custom == null && overlay == null) return false
        val callback = customCallback
        customCallback = null
        custom = null
        overlay?.let { host ->
            (host.parent as? ViewGroup)?.removeView(host)
            host.removeAllViews()
        }
        overlay = null
        activity.window.decorView.systemUiVisibility = priorSystemUi
        callback?.onCustomViewHidden()
        event("fullscreen", false)
        return true
    }

    private fun releaseWeb() {
        exitFullscreen()
        val player = web
        web = null
        player?.apply {
            setOnTouchListener(null)
            webChromeClient = null
            webViewClient = WebViewClient()
            stopLoading()
            loadUrl("about:blank")
            onPause()
            (parent as? ViewGroup)?.removeView(this)
            removeAllViews()
            destroy()
        }
    }

    fun stop(notify: Boolean = false) {
        handler.removeCallbacksAndMessages(null)
        releaseWeb()
        if (notify) event("stopped")
    }

    override fun dispose() {
        if (disposed) return
        stop()
        disposed = true
        channel?.setMethodCallHandler(null)
        frame.removeAllViews()
        onDisposed?.invoke(this)
        onDisposed = null
    }
}
