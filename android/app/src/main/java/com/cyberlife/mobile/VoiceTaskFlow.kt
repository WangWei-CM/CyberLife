package com.cyberlife.mobile

import android.content.Context
import android.content.Intent
import android.speech.RecognizerIntent
import java.util.Locale

/**
 * Android-only voice task flow. The host UI should call startRecognition,
 * allow editing the transcript, then send the text to the LLM adapter.
 * LLM output must be validated against TaskDraft before API submission.
 */
data class TaskDraft(
    val title: String,
    val dueDate: String?,
    val priority: String,
    val description: String,
)

object VoiceTaskFlow {
    fun startRecognition(context: Context): Intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
        putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
        putExtra(RecognizerIntent.EXTRA_LANGUAGE, Locale.SIMPLIFIED_CHINESE.toLanguageTag())
        putExtra(RecognizerIntent.EXTRA_PROMPT, "请说出要添加的任务")
    }

    fun validateDraft(draft: TaskDraft): Boolean =
        draft.title.isNotBlank() && draft.priority in setOf("low", "normal", "high") &&
            (draft.dueDate == null || Regex("\\d{4}-\\d{2}-\\d{2}").matches(draft.dueDate))
}
