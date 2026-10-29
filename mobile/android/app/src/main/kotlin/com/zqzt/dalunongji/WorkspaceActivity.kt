package com.zqzt.dalunongji

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.graphics.Color
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.util.Base64
import android.view.View
import android.view.WindowManager
import android.view.WindowInsets
import android.graphics.Rect
import android.view.inputmethod.InputMethodManager
import android.webkit.*
import android.widget.*
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.util.UUID

/** Latest bundled React UI, same API origin and authorization as the native app.
 * No JavaScript interface, file:// access or token-bearing URLs are exposed. */
class WorkspaceActivity : Activity() {
    private lateinit var web: WebView
    private lateinit var base: Uri
    private lateinit var payload: JSONObject
    private val nonce = UUID.randomUUID().toString()
    private var picker: ValueCallback<Array<Uri>>? = null
    private var pendingBytes: ByteArray? = null
    private var closing = false
    private var previewDialog: AlertDialog? = null
    private val allowedFiles = setOf("image/jpeg", "image/png", "image/webp", "application/pdf", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
    private fun trusted(url: String?): Boolean = try {
        val u = Uri.parse(url); u.scheme == base.scheme && u.encodedAuthority == base.encodedAuthority && u.userInfo == null
    } catch (_: Exception) { false }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        try {
            payload = JSONObject(intent.getStringExtra("payload") ?: error("missing payload"))
            base = Uri.parse(payload.getString("baseUrl"))
            require(base.scheme in listOf("http", "https") && !base.host.isNullOrEmpty() && base.userInfo == null && base.query == null && base.fragment == null && (base.path.isNullOrEmpty() || base.path == "/"))
            val debug = applicationInfo.flags and android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE != 0
            require(debug || base.toString().trimEnd('/') == "https://zqzhg.com")
            val route = payload.getString("path")
            require(route.startsWith("/") && !route.startsWith("//") && !route.contains('\\') && !route.any { it.code <= 32 })
            require(Uri.parse(route).path == "/regions")
            val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setBackgroundColor(Color.WHITE) }
            val bar = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL; gravity = android.view.Gravity.CENTER_VERTICAL }
            val english = payload.optString("language") == "en"
            val back = Button(this).apply { text = "‹"; textSize = 28f; setPadding(0, 0, 0, 0); includeFontPadding = false; setBackgroundColor(Color.TRANSPARENT); setTextColor(Color.rgb(23,43,66)); contentDescription = if (english) "Back" else "返回"; setOnClickListener { this@WorkspaceActivity.back() } }
            val title = TextView(this).apply { text = payload.optString("title", "大陆农机"); textSize = 17f; maxLines = 2; ellipsize = android.text.TextUtils.TruncateAt.END; gravity = android.view.Gravity.CENTER_VERTICAL; setTextColor(Color.rgb(23,43,66)) }
            val language = Button(this).apply { text = if (english) "中文" else "English"; textSize = 13f; setPadding(dp(4), 0, dp(4), 0); isAllCaps = false; setBackgroundColor(Color.TRANSPARENT); setTextColor(Color.rgb(23,105,210)); setOnClickListener { web.evaluateJavascript("Boolean(window.__DLNJ_TOGGLE_LANGUAGE__&&(window.__DLNJ_TOGGLE_LANGUAGE__(),true))") { changed -> if (changed == "true") { val next = if (text == "中文") "zh" else "en"; payload.put("language", next); text = if (next == "en") "中文" else "English"; back.contentDescription = if (next == "en") "Back" else "返回" } } } }
            bar.minimumHeight = dp(56)
            bar.addView(back, LinearLayout.LayoutParams(dp(48), dp(56)))
            bar.addView(title, LinearLayout.LayoutParams(0, -2, 1f))
            bar.addView(language, LinearLayout.LayoutParams(maxOf(dp(84), (dp(72) * resources.configuration.fontScale).toInt()), dp(56)))
            val progress = ProgressBar(this, null, android.R.attr.progressBarStyleHorizontal)
            web = WebView(this)
            root.addView(bar); root.addView(progress, LinearLayout.LayoutParams(-1, dp(3))); root.addView(web, LinearLayout.LayoutParams(-1, 0, 1f))
            root.setOnApplyWindowInsetsListener { v, insets ->
                @Suppress("DEPRECATION")
                v.setPadding(insets.systemWindowInsetLeft, insets.systemWindowInsetTop, insets.systemWindowInsetRight, insets.systemWindowInsetBottom)
                insets
            }
            setContentView(root)
            window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE)
            // Private procurement and quotes should not enter Android recent-task snapshots.
            window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
            WebView.setWebContentsDebuggingEnabled(false)
            web.settings.apply {
                javaScriptEnabled = true; domStorageEnabled = true
                allowFileAccess = false; allowContentAccess = false
                cacheMode = WebSettings.LOAD_NO_CACHE
                mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                mediaPlaybackRequiresUserGesture = true
                setSupportMultipleWindows(false)
            }
            CookieManager.getInstance().setAcceptThirdPartyCookies(web, false)
            web.webViewClient = object : WebViewClient() {
                override fun shouldInterceptRequest(view: WebView, req: WebResourceRequest): WebResourceResponse? {
                    if (!trusted(req.url.toString()) || req.method != "GET") return null
                    val path = req.url.path ?: return null
                    if (path.startsWith("/api/")) return null
                    if (path.contains('\\') || path.split('/').any { it == ".." }) return blocked()
                    val resource = when {
                        path.startsWith("/assets/") || path.startsWith("/pdfjs/") || path.startsWith("/maps/") || path == "/brand.svg" -> path.trimStart('/')
                        routePath(path) -> "index.html"
                        else -> return blocked()
                    }
                    return try {
                        if (resource == "index.html") {
                            val html = assets.open("workspace/index.html").bufferedReader().use { it.readText() }
                            WebResourceResponse("text/html", "UTF-8", ByteArrayInputStream(inject(html).toByteArray(Charsets.UTF_8))).apply { responseHeaders = mapOf("Cache-Control" to "no-store", "X-Robots-Tag" to "noindex") }
                        } else WebResourceResponse(mime(resource), if (resource.endsWith(".js") || resource.endsWith(".css") || resource.endsWith(".json")) "UTF-8" else null, assets.open("workspace/$resource"))
                    } catch (_: Exception) { blocked() }
                }
                override fun shouldOverrideUrlLoading(view: WebView, req: WebResourceRequest): Boolean {
                    if (trusted(req.url.toString())) return false
                    if (req.isForMainFrame && req.hasGesture() && req.url.scheme in listOf("https", "http", "tel")) {
                        try { startActivity(Intent(Intent.ACTION_VIEW, req.url)) } catch (_: Exception) { toast("无法打开链接") }
                    }
                    return true
                }
                override fun onReceivedError(view: WebView, req: WebResourceRequest, error: WebResourceError) {
                    if (req.isForMainFrame) toast("页面加载失败，请返回后重试")
                }
                override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: android.net.http.SslError) { handler.cancel(); toast("服务器证书无效，已停止连接") }
            }
            web.webChromeClient = object : WebChromeClient() {
                override fun onReceivedTitle(view: WebView, value: String?) { if (trusted(view.url) && !value.isNullOrBlank()) title.text = value }
                override fun onProgressChanged(view: WebView, value: Int) { progress.progress = value; progress.visibility = if (value == 100) View.GONE else View.VISIBLE }
                override fun onJsPrompt(view: WebView, url: String, message: String, value: String?, result: JsPromptResult): Boolean {
                    if (message == nonce && trusted(url)) {
                        try { download(JSONObject(value ?: "{}")); result.confirm("ok") } catch (_: Exception) { result.cancel(); toast("附件无法保存，或超过20MB限制") }
                    } else result.cancel()
                    return true
                }
                override fun onJsBeforeUnload(view: WebView, url: String, message: String, result: JsResult): Boolean {
                    AlertDialog.Builder(this@WorkspaceActivity).setMessage(words("有未保存内容，确定离开？", "Leave with unsaved changes?")).setNegativeButton(words("继续编辑", "Keep editing")) { _, _ -> result.cancel() }.setPositiveButton(words("离开", "Leave")) { _, _ -> result.confirm() }.setOnCancelListener { result.cancel() }.show(); return true
                }
                override fun onShowFileChooser(view: WebView, callback: ValueCallback<Array<Uri>>, params: FileChooserParams): Boolean {
                    picker?.onReceiveValue(null); picker = callback
                    try { startActivityForResult(params.createIntent().apply { addCategory(Intent.CATEGORY_OPENABLE) }, 5101) } catch (_: Exception) { picker?.onReceiveValue(null); picker = null; toast("无法打开文件选择器") }
                    return true
                }
            }
            web.loadUrl(base.toString().trimEnd('/') + route)
        } catch (_: Exception) { setResult(RESULT_CANCELED); finish() }
    }

    private fun inject(html: String): String {
        val config = payload.getJSONObject("config")
        val root = base.toString().trimEnd('/')
        config.put("version", maxOf(1, config.optInt("version", 1)))
        val site = config.optJSONObject("site") ?: JSONObject()
        site.put("name", "中企展馆").put("name_en", "China Enterprise Exhibition").put("web_url", root).put("api_url", "$root/api/v1").put("media_url", "$root/api/v1/media")
        config.put("site", site)
        fun safe(s: String) = s.replace("<", "\\u003c").replace(">", "\\u003e").replace("\u2028", "\\u2028").replace("\u2029", "\\u2029")
        val runtime = "<script id=\"zqzg-runtime-config\" type=\"application/json\">${safe(config.toString())}</script>"
        val seed = JSONObject().put("token", payload.optString("token")).put("account", payload.optJSONObject("account") ?: JSONObject()).put("language", payload.optString("language", "zh")).put("nonce", nonce)
        val bridge = assets.open("native-workspace.js").bufferedReader().use { it.readText() }
        val bootstrap = "<script>" + bridge.replace("__DLNJ_BOOTSTRAP__", safe(seed.toString())) + "</script>"
        val mapStyle = assets.open("native-map.css").bufferedReader().use { it.readText() }
        return html.replace("<head>", "<head>$runtime$bootstrap<style id=\"native-map-layout\">$mapStyle</style>")
    }
    private fun routePath(path: String) = path == "/" || Regex("^/procurement(?:/[1-9][0-9]*)?$").matches(path) || path in listOf("/join", "/app", "/browse", "/browse/products", "/regions", "/merchant") || listOf("/buyer", "/merchant/", "/products/", "/vendors/", "/v/").any { path == it || path.startsWith(if (it.endsWith('/')) it else "$it/") }
    private fun mime(path: String): String = when (path.substringAfterLast('.')) { "js", "mjs" -> "text/javascript"; "css" -> "text/css"; "json" -> "application/json"; "wasm" -> "application/wasm"; "svg" -> "image/svg+xml"; else -> android.webkit.MimeTypeMap.getSingleton().getMimeTypeFromExtension(path.substringAfterLast('.')) ?: "application/octet-stream" }
    private fun blocked() = WebResourceResponse("text/plain", "UTF-8", 404, "Not Found", mapOf("Cache-Control" to "no-store"), ByteArrayInputStream(ByteArray(0)))
    private fun dp(n: Int) = (n * resources.displayMetrics.density).toInt()
    private fun words(zh: String, en: String) = if (payload.optString("language") == "en") en else zh
    private fun toast(s: String) = runOnUiThread { Toast.makeText(this, s, Toast.LENGTH_LONG).show() }
    private fun download(value: JSONObject) {
        require(pendingBytes == null)
        val type = value.getString("mime").substringBefore(';')
        require(type in allowedFiles)
        val data = value.getString("data"); require(data.length <= 28 * 1024 * 1024 && data.startsWith("data:") && data.substringBefore(',').endsWith(";base64"))
        val bytes = Base64.decode(data.substringAfter(','), Base64.DEFAULT); require(bytes.size <= 20 * 1024 * 1024)
        if (value.optBoolean("preview") && (type.startsWith("image/") || type == "application/pdf")) { preview(bytes, type); return }
        val extension = when(type) { "image/jpeg" -> "jpg"; "image/png" -> "png"; "image/webp" -> "webp"; "application/pdf" -> "pdf"; "application/vnd.openxmlformats-officedocument.wordprocessingml.document" -> "docx"; else -> "xlsx" }
        val name = value.optString("name", "attachment").replace(Regex("[\\\\/\\p{Cntrl}]"), "_").take(160).ifBlank { "attachment" }
        pendingBytes = bytes
        try { startActivityForResult(Intent(Intent.ACTION_CREATE_DOCUMENT).apply { addCategory(Intent.CATEGORY_OPENABLE); this.type = type; putExtra(Intent.EXTRA_TITLE, if (name.contains('.')) name else "$name.$extension") }, 5102) } catch (e: Exception) { pendingBytes = null; throw e }
    }
    private fun preview(bytes: ByteArray, type: String) {
        previewDialog?.dismiss()
        val layout = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        val image = ImageView(this).apply { scaleType = ImageView.ScaleType.FIT_CENTER; setBackgroundColor(Color.WHITE) }
        layout.addView(image, LinearLayout.LayoutParams(-1, (resources.displayMetrics.heightPixels * 0.55).toInt()))
        var temp: java.io.File? = null
        var descriptor: android.os.ParcelFileDescriptor? = null
        var pdf: android.graphics.pdf.PdfRenderer? = null
        try {
            if (type == "application/pdf") {
                val file = java.io.File.createTempFile("procurement-", ".pdf", cacheDir).apply { writeBytes(bytes) }; temp = file
                val fd = android.os.ParcelFileDescriptor.open(file, android.os.ParcelFileDescriptor.MODE_READ_ONLY); descriptor = fd
                val renderer = android.graphics.pdf.PdfRenderer(fd); pdf = renderer
                require(renderer.pageCount > 0)
                val controls = LinearLayout(this)
                val previous = Button(this).apply { text = "上一页" }; val next = Button(this).apply { text = "下一页" }; val count = TextView(this)
                controls.addView(previous); controls.addView(count, LinearLayout.LayoutParams(0, -2, 1f)); controls.addView(next); layout.addView(controls)
                var index = 0
                fun render() {
                    try { renderer.openPage(index).use { page ->
                        val scale = minOf(1200.0 / page.width, 1600.0 / page.height)
                        val bitmap = android.graphics.Bitmap.createBitmap(maxOf(1, (page.width * scale).toInt()), maxOf(1, (page.height * scale).toInt()), android.graphics.Bitmap.Config.ARGB_8888)
                        bitmap.eraseColor(Color.WHITE); page.render(bitmap, null, null, android.graphics.pdf.PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY); image.setImageBitmap(bitmap)
                    } } catch (_: Exception) { toast("此页无法预览"); return }
                    count.text = "${index + 1} / ${renderer.pageCount}"; previous.isEnabled = index > 0; next.isEnabled = index + 1 < renderer.pageCount
                }
                previous.setOnClickListener { index--; render() }; next.setOnClickListener { index++; render() }; render()
            } else {
                val bounds = android.graphics.BitmapFactory.Options().apply { inJustDecodeBounds = true }
                android.graphics.BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bounds)
                var sample = 1
                while (maxOf(bounds.outWidth, bounds.outHeight) / sample > 1600) sample *= 2
                val bitmap = android.graphics.BitmapFactory.decodeByteArray(bytes, 0, bytes.size, android.graphics.BitmapFactory.Options().apply { inSampleSize = sample }) ?: error("Invalid image")
                image.setImageBitmap(bitmap)
            }
            val dialog = AlertDialog.Builder(this).setTitle("附件预览").setView(layout).setNegativeButton("关闭", null).create()
            dialog.setOnDismissListener { pdf?.close(); descriptor?.close(); temp?.delete(); previewDialog = null }
            previewDialog = dialog; dialog.show(); dialog.window?.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        } catch (_: Exception) { pdf?.close(); descriptor?.close(); temp?.delete(); toast("无法预览该附件，请下载查看") }
    }
    private fun back() {
        val rect = Rect(); web.getWindowVisibleDisplayFrame(rect)
        val keyboard = if (Build.VERSION.SDK_INT >= 30) web.rootWindowInsets?.isVisible(WindowInsets.Type.ime()) == true else web.rootView.height - rect.bottom > dp(150)
        if (keyboard) { (getSystemService(INPUT_METHOD_SERVICE) as InputMethodManager).hideSoftInputFromWindow(web.windowToken, 0); return }
        if (!trusted(web.url)) { close(); return }
        web.evaluateJavascript("Boolean(window.__DLNJ_BACK__&&window.__DLNJ_BACK__())") { handled ->
            if (handled != "true") { if (web.canGoBack()) web.goBack() else close() }
        }
    }
    private fun close() {
        if (closing) return
        web.evaluateJavascript("(()=>{const e=new Event('beforeunload',{cancelable:true});window.dispatchEvent(e);return e.defaultPrevented})()") { raw ->
            if (raw == "true") AlertDialog.Builder(this).setMessage(words("采购或报价内容尚未保存，确定关闭？", "Close with unsaved request or quotation changes?")).setNegativeButton(words("继续编辑", "Keep editing"), null).setPositiveButton(words("关闭", "Close")) { _, _ -> finishSession() }.show()
            else finishSession()
        }
    }
    private fun finishSession() {
        if (closing) return
        closing = true
        if (!trusted(web.url)) { setResult(RESULT_CANCELED); finish(); return }
        web.evaluateJavascript("JSON.stringify({token:sessionStorage.getItem('zqzt.merchant')||'',language:localStorage.getItem('zqzt.language')||'zh'})") { raw ->
            try { val s = org.json.JSONTokener(raw).nextValue() as String; val state = JSONObject(s); setResult(RESULT_OK, Intent().putExtra("token", state.getString("token")).putExtra("language", state.getString("language"))) } catch (_: Exception) { setResult(RESULT_CANCELED) }
            finish()
        }
    }
    @Deprecated("Deprecated in Java") override fun onBackPressed() { back() }
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == 5101) { picker?.onReceiveValue(WebChromeClient.FileChooserParams.parseResult(resultCode, data)); picker = null }
        if (requestCode == 5102) { val bytes = pendingBytes; pendingBytes = null; if (resultCode == RESULT_OK && bytes != null && data?.data != null) try { contentResolver.openOutputStream(data.data!!, "w")?.use { it.write(bytes) }; toast("附件已保存") } catch (_: Exception) { toast("保存失败，请重试") } }
    }
    override fun onDestroy() {
        previewDialog?.dismiss()
        picker?.onReceiveValue(null); picker = null; pendingBytes = null
        if (::web.isInitialized) { web.stopLoading(); web.clearHistory(); web.destroy() }
        super.onDestroy()
    }
}
