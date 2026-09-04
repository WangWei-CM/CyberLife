package com.cyberlife.mobile

import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONArray

class PollApi(private val baseUrl: String, private val deviceToken: String) {
    private val client = OkHttpClient()
    fun poll(cursor: String?): PollResult {
        val url = baseUrl.trimEnd('/') + "/api/v1/mobile/poll" + (cursor?.let { "?cursor=$it" } ?: "")
        val request = Request.Builder().url(url).header("Authorization", "Bearer $deviceToken").get().build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) error("poll failed: ${response.code}")
            val body = response.body?.string().orEmpty()
            val json = org.json.JSONObject(body)
            val events = json.optJSONArray("events") ?: JSONArray()
            val ids = (0 until events.length()).mapNotNull { events.optJSONObject(it)?.optString("event_id") }
            return PollResult(json.optString("next_cursor", cursor), ids)
        }
    }
}

data class PollResult(val nextCursor: String?, val eventIds: List<String>)
