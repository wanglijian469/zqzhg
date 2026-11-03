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
    private val loadState = OfficialVideoLoadState()
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
    private fun event(name: String, value: Any? = null) {
        if (!disposed) {
            channel?.invokeMethod(name, value)
            onEvent?.invoke(name, value)
        }
    }

    private fun detail(reason: String, raw: String = "", code: Int = 0, mainFrame: Boolean = false): Map<String, Any> =
        mapOf("reason" to reason, "host" to OfficialVideoPolicy.diagnosticHost(raw), "scheme" to OfficialVideoPolicy.diagnosticScheme(raw), "scope" to if (mainFrame) "main" else "resource", "code" to code)

    private fun fail(reason: String, raw: String = "", code: Int = 0, mainFrame: Boolean = false) {
        if (disposed || failed) return
        failed = true
        loadState.stop()
        handler.removeCallbacksAndMessages(null)
        releaseWeb()
        event("error", detail(reason, raw, code, mainFrame))
    }

    private fun isPlayerRequest(request: WebResourceRequest): Boolean =
        request.isForMainFrame || OfficialVideoPolicy.criticalResource(request.url.toString())

    private fun load() {
        handler.removeCallbacksAndMessages(null)
        releaseWeb()
        failed = false
        lastUserTouch = Long.MIN_VALUE
        event("loading")
        if (!trustedPlayer(rawUrl)) { fail("invalid_player"); return }
        val document = OfficialVideoDocument(rawUrl, wide, UUID.randomUUID().toString())
        val attempt = loadState.begin()
        val player = WebView(activity)
        web = player
        // A slow network must not destroy a still-loading official iframe.
        // No automatic reload: the user can keep waiting, retry, or close it.
        val loadTimeout = Runnable {
            if (web === player && !disposed && loadState.slow(attempt)) event("slow", detail("slow_load"))
        }
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
                document.route(request.url.toString(), request.isForMainFrame) == OfficialVideoDocument.BLOCK

            override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
                return when (document.route(request.url.toString(), request.isForMainFrame)) {
                    OfficialVideoDocument.DOCUMENT -> WebResourceResponse(
                        "text/html", "UTF-8", 200, "OK",
                        mapOf("Cache-Control" to "no-store", "X-Content-Type-Options" to "nosniff"),
                        ByteArrayInputStream(document.html.toByteArray(Charsets.UTF_8)),
                    )
                    OfficialVideoDocument.NETWORK -> null
                    else -> WebResourceResponse("text/plain", "UTF-8", 403, "Blocked by player policy", mapOf("Cache-Control" to "no-store", "X-ZQZG-Player-Policy" to "blocked"), ByteArrayInputStream(ByteArray(0)))
                }
            }

            override fun onPageFinished(view: WebView, url: String) {
                if (web === view && !failed && loadState.ready(attempt)) {
                    handler.removeCallbacks(loadTimeout)
                    event("loaded")
                }
            }

            override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                if (isPlayerRequest(request)) handler.post {
                    if (web === view) fail("network", request.url.toString(), error.errorCode, request.isForMainFrame)
                }
            }

            override fun onReceivedHttpError(view: WebView, request: WebResourceRequest, response: WebResourceResponse) {
                if (isPlayerRequest(request) && response.statusCode >= 400) handler.post {
                    if (web === view) {
                        val policyBlocked = response.responseHeaders?.get("X-ZQZG-Player-Policy") == "blocked" || response.reasonPhrase == "Blocked by player policy"
                        fail(if (policyBlocked) "app_policy" else "http", request.url.toString(), response.statusCode, request.isForMainFrame)
                    }
                }
            }

            override fun onReceivedSslError(view: WebView, ssl: SslErrorHandler, error: android.net.http.SslError) {
                ssl.cancel()
                // Reject every bad certificate. A failed optional image/telemetry request
                // is not evidence that the player document or its scripts cannot load.
                handler.post {
                    if (web === view) {
                        if (OfficialVideoPolicy.failureIsFatal("tls", false, error.url)) {
                            fail("tls", error.url, error.primaryError)
                        } else {
                            event("resource-warning", detail("optional_tls", error.url, error.primaryError))
                        }
                    }
                }
            }

            override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
                handler.post { if (web === view) fail("renderer") }
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
        // Serve exactly this app-generated main document locally. Do not feed a data:
        // navigation into the remote-resource filter or request this route from ECS.
        player.loadUrl(document.url)
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
        loadState.stop()
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
