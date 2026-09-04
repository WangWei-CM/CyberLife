package com.cyberlife.mobile

import android.app.Activity
import android.os.Bundle
import android.widget.TextView

class MainActivity : Activity() {
    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        setContentView(TextView(this).apply { text = "Cyberlife 通知与语音助手\n后台轮询已准备"; textSize = 18f; setPadding(32, 64, 32, 32) })
        PollWorker.enqueue(this)
    }
}
