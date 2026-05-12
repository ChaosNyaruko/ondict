plugins {
    alias(libs.plugins.android.application)
}

// ---------- gomobile build configuration ----------
val ondictRepoDir = file("../../")              // relative: android/app/ -> repo root
val goPath         = "/Users/yqg/go"
val androidHome    = "/Users/yqg/Library/Android/sdk"
val ndkVersion     = "30.0.14904198"
val gomobileBin    = "$goPath/bin/gomobile"
val outputAar      = layout.projectDirectory.file("libs/mobile.aar").asFile

tasks.register<Exec>("gomobileBind") {
    description = "Compile the Go mobile package into mobile.aar using gomobile bind"
    group       = "build"

    workingDir = ondictRepoDir
    environment("ANDROID_HOME",     androidHome)
    environment("ANDROID_NDK_HOME", "$androidHome/ndk/$ndkVersion")
    environment("PATH",             "${System.getenv("PATH")}:$goPath/bin")
    environment("HOME",             System.getenv("HOME") ?: "")

    commandLine(
        gomobileBin, "bind",
        "-target", "android/arm64",
        "-androidapi", "36",
        "-o", outputAar.absolutePath,
        "./mobile/"
    )

    inputs.dir(ondictRepoDir.resolve("mobile"))
    inputs.dir(ondictRepoDir.resolve("sources"))
    inputs.dir(ondictRepoDir.resolve("decoder"))
    inputs.dir(ondictRepoDir.resolve("render"))
    inputs.dir(ondictRepoDir.resolve("util"))
    inputs.dir(ondictRepoDir.resolve("internal"))
    inputs.dir(ondictRepoDir.resolve("wordbank"))
    inputs.dir(ondictRepoDir.resolve("history"))
    outputs.file(outputAar)
}

// Run gomobileBind automatically before every Android build
tasks.whenTaskAdded {
    if (name == "preBuild") {
        dependsOn("gomobileBind")
    }
}
// --------------------------------------------------

android {
    namespace = "com.ondict.app"
    compileSdk {
        version = release(36) {
            minorApiLevel = 1
        }
    }

    defaultConfig {
        applicationId = "com.ondict.app"
        minSdk = 36
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
}

dependencies {
    implementation(fileTree(mapOf("dir" to "libs", "include" to listOf("*.aar", "*.jar"))))
    implementation(libs.androidx.activity.ktx)
    implementation(libs.androidx.appcompat)
    implementation(libs.androidx.constraintlayout)
    implementation(libs.androidx.core.ktx)
    implementation(libs.material)
    testImplementation(libs.junit)
    androidTestImplementation(libs.androidx.espresso.core)
    androidTestImplementation(libs.androidx.junit)
}
