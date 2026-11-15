import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// Versão: a mesma do app (internal/app/version.go). O CI passa por
// -PradlogVersion=3.0.0-alpha.1 e -PradlogCode=…; sem elas, vale a de teste.
val radlogVersion = (findProperty("radlogVersion") as String?) ?: "3.0.0-dev"
val radlogCode = ((findProperty("radlogCode") as String?) ?: "1").toInt()

// Assinatura: o keystore do RadLog vem dos segredos do GitHub (o CI escreve
// o arquivo e passa os caminhos). Sem eles, o APK sai com a chave de debug.
val sign = Properties().apply {
    listOf("storeFile", "storePassword", "keyAlias", "keyPassword").forEach { k ->
        (findProperty("radlog.$k") as String?)?.let { setProperty(k, it) }
    }
}

android {
    namespace = "app.radlog"
    compileSdk = 36

    defaultConfig {
        applicationId = "app.radlog"
        minSdk = 29
        targetSdk = 36
        versionCode = radlogCode
        versionName = radlogVersion
    }

    signingConfigs {
        if (sign.getProperty("storeFile") != null) {
            create("radlog") {
                storeFile = file(sign.getProperty("storeFile"))
                storePassword = sign.getProperty("storePassword")
                keyAlias = sign.getProperty("keyAlias")
                keyPassword = sign.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.findByName("radlog") ?: signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    // só arm64 (o motor em Go é compilado só para ele)
    splits { abi { isEnable = false } }

    sourceSets["main"].assets.srcDir(layout.buildDirectory.dir("generated/radlogAssets"))

    packaging { jniLibs { useLegacyPackaging = false } }
}

kotlin { jvmToolchain(17) }

// fontes IBM Plex: as mesmas do desktop (frontend/dist/fonts)
val copyFonts by tasks.registering(Copy::class) {
    from(rootProject.file("../frontend/dist/fonts")) { include("*.woff2", "OFL-IBM-Plex.txt") }
    into(layout.buildDirectory.dir("generated/radlogAssets/www/fonts"))
}
tasks.named("preBuild") { dependsOn(copyFonts) }

dependencies {
    implementation(files("libs/radlog.aar"))
    implementation("androidx.core:core-ktx:1.16.0")
    implementation("androidx.webkit:webkit:1.14.0")
    implementation("androidx.browser:browser:1.8.0")
}
