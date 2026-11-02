package com.zqzt.dalunongji

import android.app.Activity
import android.graphics.Color
import android.content.Intent
import android.net.Uri
import android.widget.ProgressBar
import android.os.Bundle
import android.view.View
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView

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
            onEvent = { name, _ ->
                when (name) {
                    "loaded" -> {
                        progress.visibility = View.GONE
                        retry.isEnabled = true
                        status.text = "请点击画面中的播放按钮。没有画面时，可重新加载或打开原视频。"
                    }
                    "error" -> {
                        progress.visibility = View.GONE
                        retry.isEnabled = true
                        status.text = "视频暂时无法加载，请重新加载，或打开抖音原视频。"
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
