package com.cyberlife.mobile

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import java.util.concurrent.TimeUnit

class PollWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        // Phase 7 contract placeholder: poll /api/v1/mobile/poll with device token,
        // de-duplicate event_id, post Android notifications, then schedule the next poll.
        return Result.success()
    }
    companion object {
        fun enqueue(context: Context) {
            val request = OneTimeWorkRequestBuilder<PollWorker>()
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
                .setInitialDelay(15, TimeUnit.MINUTES).build()
            WorkManager.getInstance(context).enqueue(request)
        }
    }
}
