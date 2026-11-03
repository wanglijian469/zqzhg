package com.zqzt.dalunongji

import android.app.Activity
import android.content.ClipData
import android.content.ClipboardManager
import android.graphics.Color
import android.content.Intent
import android.net.Uri
import android.widget.ProgressBar
import android.os.Bundle
import android.os.Build
import android.view.View
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import android.webkit.WebView
import android.webkit.WebSettings

/** Dedicated token-free window using the same trusted WebView backend as platform views. */
class VideoPlayerActivity : Activity() {
    private var player: OfficialVideoView? = null

    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(Color.BLACK)
        }
        layout.addView(Button(this).apply {
            text = "关闭视频"
            setOnClickListener { finish() }
        })
        val status = TextView(this).apply {
            text = "正在加载抖音视频…"
            setTextColor(Color.WHITE)
            setPadding(16, 8, 16, 8)
        }
        layout.addView(status)
        val progress = ProgressBar(this)
        layout.addView(progress)
        val retry = Button(this).apply {
            text = "重新加载视频"
            isEnabled = false
            setOnClickListener {
                isEnabled = false
                progress.visibility = View.VISIBLE
                status.text = "正在加载抖音视频…"
                player?.start()
            }
        }
        layout.addView(retry)
        val diagnostic = TextView(this).apply {
            visibility = View.GONE
            setTextColor(Color.LTGRAY)
            textSize = 12f
            setPadding(16, 8, 16, 12)
        }
        val details = Button(this).apply {
            text = "查看播放诊断"
            visibility = View.GONE
            setOnClickListener {
                diagnostic.visibility = if (diagnostic.visibility == View.VISIBLE) View.GONE else View.VISIBLE
            }
        }
        layout.addView(details)
        layout.addView(diagnostic)
        diagnostic.setOnClickListener {
            val clipboard = getSystemService(CLIPBOARD_SERVICE) as ClipboardManager
            clipboard.setPrimaryClip(ClipData.newPlainText("播放诊断", diagnostic.text))
            details.text = "诊断已复制"
        }
        val original = OfficialVideoPolicy.originalVideoURL(intent.getStringExtra("url") ?: "")
        if (original.isNotEmpty()) layout.addView(Button(this).apply {
            text = "打开抖音原视频"
            setOnClickListener {
                try { startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(original))) }
                catch (_: Exception) { status.text = "无法打开原视频，请稍后重试。" }
            }
        })
        val view = OfficialVideoView(
            this, null, -1,
            mapOf(
                "url" to (intent.getStringExtra("url") ?: ""),
                "width" to intent.getIntExtra("width", 0),
                "height" to intent.getIntExtra("height", 0),
            ),
            onEvent = { name, value ->
                when (name) {
                    "loading" -> {
                        progress.visibility = View.VISIBLE
                        retry.isEnabled = false
                        status.text = "正在加载抖音视频…"
                        details.visibility = View.GONE
                        details.text = "查看播放诊断"
                        diagnostic.visibility = View.GONE
                        diagnostic.text = ""
                    }
                    "loaded" -> {
                        progress.visibility = View.GONE
                        retry.isEnabled = true
                        status.text = "请点击画面中的播放按钮。没有画面时，可重新加载或打开原视频。"
                        if (diagnostic.text.isEmpty()) diagnostic.text = playbackDiagnostic(mapOf("reason" to "document_loaded"))
                        details.visibility = View.VISIBLE
                    }
                    "error" -> {
                        progress.visibility = View.GONE
                        retry.isEnabled = true
                        val info = value as? Map<*, *> ?: emptyMap<Any, Any>()
                        val reason = info["reason"] as? String ?: "unknown"
                        val code = (info["code"] as? Number)?.toInt() ?: 0
                        status.text = when {
                            reason == "tls" || reason == "network" && code == -11 -> "手机与抖音的安全连接失败。请检查手机时间和系统浏览组件更新，或打开原视频。"
                            reason == "network" && code == -2 -> "手机暂时找不到抖音的播放服务，请切换网络后重试。"
                            reason == "network" && code == -8 -> "连接抖音超时，请切换网络后重试。"
                            reason == "http" -> "抖音播放服务暂时未能响应，请稍后重试，或打开原视频。"
                            reason == "renderer" -> "手机的网页播放组件已停止，请关闭后重新打开，或更新系统浏览组件。"
                            reason == "invalid_player" -> "播放器地址无效，请返回重新检查视频链接。"
                            else -> "视频暂时无法加载，请重新加载，或打开抖音原视频。"
                        }
                        diagnostic.text = playbackDiagnostic(info)
                        details.visibility = View.VISIBLE
                    }
                    "slow" -> {
                        progress.visibility = View.GONE
                        retry.isEnabled = true
                        status.text = "抖音加载较慢，仍在等待。可以继续等待、重新加载或打开原视频。"
                        diagnostic.text = playbackDiagnostic(value as? Map<*, *> ?: emptyMap<Any, Any>())
                        details.visibility = View.VISIBLE
                    }
                    "resource-warning" -> {
                        diagnostic.text = playbackDiagnostic(value as? Map<*, *> ?: emptyMap<Any, Any>())
                        details.visibility = View.VISIBLE
                    }
                }
            },
            onDisposed = null,
        )
        player = view
        layout.addView(view.getView(), LinearLayout.LayoutParams(-1, 0, 1f))
        setContentView(layout)
        view.start()
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
        return "手机：${Build.MANUFACTURER} ${Build.MODEL}\nAndroid：${Build.VERSION.RELEASE}\nApp：${app.versionName ?: "unknown"}（$code）\n网页播放组件：$version\n原因：${info["reason"] ?: "unknown"}\n服务：${info["host"] ?: ""}\n编号：${info["code"] ?: 0}\n点击此处复制诊断"
    }

    @Deprecated("Android back compatibility")
    override fun onBackPressed() {
        if (player?.exitFullscreen() != true) finish()
    }

    private fun release() {
        player?.dispose()
        player = null
    }

    override fun onPause() {
        release()
        if (!isFinishing) finish()
        super.onPause()
    }

    override fun onDestroy() {
        release()
        super.onDestroy()
    }
}
