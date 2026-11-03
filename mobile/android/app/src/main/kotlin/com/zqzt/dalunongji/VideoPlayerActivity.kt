package com.zqzt.dalunongji

import android.app.Activity
import android.app.AlertDialog
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Intent
import android.graphics.Color
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.view.Gravity
import android.view.View
import android.view.WindowInsets
import android.webkit.WebSettings
import android.webkit.WebView
import android.widget.Button
import android.widget.FrameLayout
import android.widget.ImageButton
import android.widget.PopupMenu
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast

/** Compact token-free player window. The verified player backend is unchanged. */
class VideoPlayerActivity : Activity() {
    private var player: OfficialVideoView? = null
    private val ui = OfficialVideoUiState()
    private var diagnosticInfo: Map<*, *> = mapOf("reason" to "loading")
    private var original = ""
    private lateinit var statusRow: View
    private lateinit var progress: View
    private lateinit var status: TextView
    private lateinit var retry: Button

    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        setContentView(R.layout.activity_official_video)
        val root = findViewById<View>(R.id.videoRoot)
        if (Build.VERSION.SDK_INT >= 30) {
            window.setDecorFitsSystemWindows(false)
            root.setOnApplyWindowInsetsListener { view, insets ->
                val safe = insets.getInsets(WindowInsets.Type.systemBars() or WindowInsets.Type.displayCutout())
                view.setPadding(safe.left, safe.top, safe.right, safe.bottom)
                insets
            }
            root.requestApplyInsets()
        } else {
            root.fitsSystemWindows = true
        }
        statusRow = findViewById(R.id.videoStatusRow)
        progress = findViewById(R.id.videoProgress)
        status = findViewById(R.id.videoStatus)
        retry = findViewById(R.id.videoRetry)
        retry.setOnClickListener { if (ui.canRetry()) player?.start() }
        findViewById<ImageButton>(R.id.videoClose).setOnClickListener { finish() }
        findViewById<ImageButton>(R.id.videoMore).setOnClickListener { showMore(it) }
        original = OfficialVideoPolicy.originalVideoURL(intent.getStringExtra("url") ?: "")
        val view = OfficialVideoView(
            this, null, -1,
            mapOf(
                "url" to (intent.getStringExtra("url") ?: ""),
                "width" to intent.getIntExtra("width", 0),
                "height" to intent.getIntExtra("height", 0),
            ),
            onEvent = ::handlePlayerEvent,
            onDisposed = null,
        )
        player = view
        findViewById<FrameLayout>(R.id.videoContainer).addView(view.getView(), FrameLayout.LayoutParams(-1, -1))
        view.start()
    }

    private fun handlePlayerEvent(name: String, value: Any?) {
        ui.event(name)
        val info = value as? Map<*, *> ?: emptyMap<Any, Any>()
        when (name) {
            "loading" -> {
                diagnosticInfo = mapOf("reason" to "loading")
                status.text = "正在加载视频…"
            }
            "loaded" -> if (diagnosticInfo["reason"] == "loading") {
                diagnosticInfo = mapOf("reason" to "document_loaded")
            }
            "error" -> {
                diagnosticInfo = info
                status.text = errorMessage(info)
            }
            "slow" -> {
                diagnosticInfo = info
                status.text = "加载较慢，可以继续等待或重试。"
            }
            "resource-warning" -> diagnosticInfo = info
        }
        statusRow.visibility = if (ui.statusVisible()) View.VISIBLE else View.GONE
        progress.visibility = if (ui.busy()) View.VISIBLE else View.GONE
        retry.visibility = if (ui.retryVisible()) View.VISIBLE else View.GONE
        retry.isEnabled = ui.canRetry()
    }

    private fun showMore(anchor: View) {
        PopupMenu(this, anchor, Gravity.END).apply {
            menu.add(0, 1, 0, "重新加载").isEnabled = ui.canRetry()
            if (original.isNotEmpty()) menu.add(0, 2, 1, "打开抖音原视频")
            menu.add(0, 3, 2, "播放诊断")
            setOnMenuItemClickListener { item ->
                when (item.itemId) {
                    1 -> if (ui.canRetry()) player?.start()
                    2 -> try {
                        startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(original)))
                    } catch (_: Exception) {
                        status.text = "暂时无法打开原视频，请稍后重试。"
                        statusRow.visibility = View.VISIBLE
                    }
                    3 -> showDiagnostic()
                }
                true
            }
            show()
        }
    }

    private fun showDiagnostic() {
        val text = playbackDiagnostic(diagnosticInfo)
        val contents = TextView(this).apply {
            this.text = text
            setTextColor(Color.LTGRAY)
            textSize = 13f
            val pad = (20 * resources.displayMetrics.density).toInt()
            setPadding(pad, pad / 2, pad, pad / 2)
            setTextIsSelectable(true)
        }
        AlertDialog.Builder(this)
            .setTitle("播放诊断")
            .setView(ScrollView(this).apply { addView(contents) })
            .setPositiveButton("复制") { _, _ ->
                (getSystemService(CLIPBOARD_SERVICE) as ClipboardManager)
                    .setPrimaryClip(ClipData.newPlainText("播放诊断", text))
                Toast.makeText(this, "诊断已复制", Toast.LENGTH_SHORT).show()
            }
            .setNegativeButton("关闭", null)
            .show()
    }

    private fun errorMessage(info: Map<*, *>): String {
        val reason = info["reason"] as? String ?: "unknown"
        val code = (info["code"] as? Number)?.toInt() ?: 0
        return when {
            reason == "tls" || reason == "network" && code == -11 -> "安全连接失败，请检查手机时间或更新系统浏览组件。"
            reason == "network" && code == -2 -> "暂时找不到播放服务，请切换网络后重试。"
            reason == "network" && code == -8 -> "连接超时，请重试。"
            reason == "app_policy" -> "播放器页面被拦截，可在更多中复制诊断。"
            reason == "http" -> "视频请求未能通过，请重试或在更多中打开原视频。"
            reason == "renderer" -> "网页播放组件已停止，请关闭后重新打开。"
            reason == "invalid_player" -> "播放器地址无效，请返回重新检查链接。"
            else -> "视频暂时无法加载，请重试或在更多中打开原视频。"
        }
    }

    private fun playbackDiagnostic(info: Map<*, *>): String {
        val version = if (Build.VERSION.SDK_INT >= 26) {
            WebView.getCurrentWebViewPackage()?.versionName ?: "unknown"
        } else {
            Regex("Chrome/([0-9.]+)").find(WebSettings.getDefaultUserAgent(this))?.groupValues?.get(1) ?: "unknown"
        }
        @Suppress("DEPRECATION")
        val app = packageManager.getPackageInfo(packageName, 0)
        @Suppress("DEPRECATION")
        val code = if (Build.VERSION.SDK_INT >= 28) app.longVersionCode else app.versionCode.toLong()
        return "手机：${Build.MANUFACTURER} ${Build.MODEL}\nAndroid：${Build.VERSION.RELEASE}\nApp：${app.versionName ?: "unknown"}（$code）\n网页播放组件：$version\n原因：${info["reason"] ?: "unknown"}\n服务：${info["host"] ?: ""}\n类型：${info["scheme"] ?: ""}\n请求：${info["scope"] ?: ""}\n编号：${info["code"] ?: 0}"
    }

    @Deprecated("Android back compatibility")
    override fun onBackPressed() {
        if (player?.exitFullscreen() != true) finish()
    }

    private fun release() { player?.dispose(); player = null }
    override fun onPause() {
        release()
        if (!isFinishing) finish()
        super.onPause()
    }
    override fun onDestroy() { release(); super.onDestroy() }
}
