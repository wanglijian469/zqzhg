package com.zqzt.dalunongji

import android.app.Activity
import android.graphics.Color
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
        layout.addView(TextView(this).apply {
            text = "抖音官方播放器：请使用播放器内的官方播放控件，是否自动播放由官方决定。"
            setTextColor(Color.WHITE)
            setPadding(16, 8, 16, 8)
        })
        val retry = Button(this).apply {
            text = "官方播放器暂时无法加载，点击重试"
            visibility = View.GONE
            setOnClickListener {
                visibility = View.GONE
                player?.start()
            }
        }
        layout.addView(retry)
        val view = OfficialVideoView(
            this, null, -1,
            mapOf(
                "url" to (intent.getStringExtra("url") ?: ""),
                "width" to intent.getIntExtra("width", 0),
                "height" to intent.getIntExtra("height", 0),
            ),
            onEvent = { name, _ -> if (name == "error") retry.visibility = View.VISIBLE },
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
