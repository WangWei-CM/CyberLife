plugins { id("com.android.application"); id("org.jetbrains.kotlin.android") }

android { namespace = "com.cyberlife.mobile"; compileSdk = 35
    defaultConfig { applicationId = "com.cyberlife.mobile"; minSdk = 26; targetSdk = 35; versionCode = 1; versionName = "0.1.0" }
}

dependencies {
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.work:work-runtime-ktx:2.10.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")
}
