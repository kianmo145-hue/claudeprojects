import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "org.paydar.app"
    compileSdk = 36

    defaultConfig {
        applicationId = "org.paydar.app"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
    }

    // Signed with the Paydar key when CI has it (ANDROID_KEYSTORE secrets), else the debug key.
    val releaseKey = System.getenv("ANDROID_KEYSTORE")
    signingConfigs {
        if (releaseKey != null) {
            create("release") {
                storeFile = file(releaseKey)
                storePassword = System.getenv("ANDROID_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("ANDROID_KEY_ALIAS")
                keyPassword = System.getenv("ANDROID_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            ndk {
                abiFilters += listOf("arm64-v8a", "armeabi-v7a")
            }
            signingConfig = signingConfigs.getByName(if (releaseKey != null) "release" else "debug")
        }
    }

    buildFeatures {
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

dependencies {
    // Built by scripts/build-go.sh (gomobile bind of ../mobile).
    implementation(files("libs/paydar.aar"))
}
