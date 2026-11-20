pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
        // a biblioteca do motor em Go (gomobile bind → app/libs/radlog.aar)
        flatDir { dirs("app/libs") }
    }
}
rootProject.name = "RadLog"
include(":app")
