# Ondict Android UI 导读

这份文档用于帮助不熟悉 Android 开发的维护者理解 Ondict Android 版的页面是如何由 XML、Kotlin、图标资源、WebView 和 Go 代码共同组成的。

当前项目使用的是经典 Android View 系统，不是 Jetpack Compose，也没有使用 View Binding。主搜索页采用“XML 布局 + Kotlin 交互”，词典内容放在 WebView 中；词典管理、单词本和同步设置页则主要由 Kotlin 直接创建控件。

## 1. 一张图看懂整体关系

```text
AndroidManifest.xml
        │ 声明启动页面、主题和应用图标
        ▼
MainActivity.kt
        │ setContentView(R.layout.activity_main)
        ▼
res/layout/activity_main.xml
        │ 创建 EditText、Button、WebView、BottomNavigationView 等 View
        ├── app:menu="@menu/bottom_nav"
        │             │
        │             └── res/menu/bottom_nav.xml
        │                         │ android:icon="@drawable/..."
        │                         └── res/drawable/ic_nav_*.xml
        │
        └── Kotlin 使用 findViewById(R.id.xxx) 取得控件
                    │
                    ├── 注册点击、输入和导航事件
                    ├── 调用 mobile.Mobile 中的 Go 接口
                    └── 把 Go 返回的 HTML 放进 WebView
```

可以把各部分简单类比为：

- XML 布局是页面的静态骨架。
- drawable XML 是可以缩放的矢量图形。
- Kotlin Activity 是页面控制器。
- `R` 是 Android 编译器生成的资源地址簿。
- Go `mobile` 包是词典、单词本、历史和同步的业务引擎。
- WebView 是专门显示 MDX 词典 HTML 的区域。

## 2. Android 目录分别放什么

```text
android/app/src/main/
├── AndroidManifest.xml
├── java/com/ondict/app/
│   ├── MainActivity.kt
│   ├── SetupActivity.kt
│   ├── WordBankActivity.kt
│   ├── SyncSettingsActivity.kt
│   └── SystemBarsAwareActivity.kt
└── res/
    ├── layout/activity_main.xml
    ├── menu/bottom_nav.xml
    ├── drawable/ic_nav_*.xml
    ├── drawable/ic_launcher_*.xml
    ├── mipmap-anydpi/ic_launcher*.xml
    ├── mipmap-*/ic_launcher*.png
    ├── values/themes.xml
    ├── values-night/themes.xml
    ├── values/colors.xml
    ├── values/strings.xml
    └── xml/backup_rules.xml 等非 UI 配置
```

常见资源目录的含义：

| 目录 | 用途 | Kotlin 中的引用形式 |
|---|---|---|
| `res/layout` | 页面布局 | `R.layout.activity_main` |
| `res/menu` | 菜单项目 | `R.menu.bottom_nav` |
| `res/drawable` | 矢量图标、背景图形 | `R.drawable.ic_nav_search` |
| `res/mipmap-*` | 应用启动图标 | 通常由 Manifest 引用 |
| `res/values` | 字符串、颜色、主题 | `R.string.*`、`R.color.*`、`R.style.*` |
| `res/xml` | 备份等通用配置 | 不一定参与页面显示 |

## 3. XML 和 Kotlin 如何通过 `R` 连接

布局文件中的这个属性：

```xml
android:id="@+id/searchButton"
```

表示声明一个名为 `searchButton` 的资源 ID。Android 构建工具编译资源后，会在应用的 `R` 类中生成相应符号：

```kotlin
R.layout.activity_main
R.id.searchButton
R.id.entryWebView
R.id.nav_wordbank
R.drawable.ic_nav_wordbank
```

其中：

- `@+id/name` 中的 `+` 表示在这里创建这个 ID。
- `@id/name` 表示引用已经存在的 ID。
- `@layout/name`、`@menu/name` 和 `@drawable/name` 分别引用对应资源目录下的文件。
- Kotlin 侧用 `R.id.name` 等形式引用相同资源。

`MainActivity` 首先加载页面：

```kotlin
setContentView(R.layout.activity_main)
```

这一步也叫 layout inflation：Android 解析 `activity_main.xml`，并为每个 XML 节点创建一个真实的 View 对象。然后 Kotlin 才能取得这些对象：

```kotlin
searchInput = findViewById(R.id.searchInput)
searchButton = findViewById(R.id.searchButton)
entryWebView = findViewById(R.id.entryWebView)
bottomNav = findViewById(R.id.bottomNav)
```

必须先调用 `setContentView`，再调用 `findViewById`，否则当前页面还没有这些 View。

相关文件：

- [`activity_main.xml`](../android/app/src/main/res/layout/activity_main.xml)
- [`MainActivity.kt`](../android/app/src/main/java/com/ondict/app/MainActivity.kt)

## 4. 应用如何进入主页面

[`AndroidManifest.xml`](../android/app/src/main/AndroidManifest.xml) 声明了应用拥有的 Activity、Service、权限、主题和应用图标。

`MainActivity` 中的 `MAIN` 和 `LAUNCHER` intent filter 表示它是桌面启动入口：

```xml
<activity
    android:name=".MainActivity"
    android:exported="true"
    android:launchMode="singleTop">
    <intent-filter>
        <action android:name="android.intent.action.MAIN" />
        <category android:name="android.intent.category.LAUNCHER" />
    </intent-filter>
</activity>
```

用户点击 Ondict 图标后，大致流程如下：

```text
Android 系统
  → 创建 MainActivity
  → 调用 MainActivity.onCreate()
  → setContentView(activity_main.xml)
  → setupWebView()
  → setupSearch()
  → setupBottomNav()
  → Mobile.init() 初始化 Go 词典引擎
```

`SetupActivity`、`WordBankActivity` 和 `SyncSettingsActivity` 也必须在 Manifest 中注册，但它们没有 `LAUNCHER` 标记，因此不会直接出现在桌面启动入口中。

## 5. 主页面的 View 层次

[`activity_main.xml`](../android/app/src/main/res/layout/activity_main.xml) 的根节点是垂直 `LinearLayout`，子控件会从上向下排列：

```text
┌──────────────────────────────┐
│ WebView 或欢迎提示            │  layout_weight=1，占据剩余空间
├──────────────────────────────┤
│ 当前词条标题 / 加入单词本按钮 │
├──────────────────────────────┤
│ 自动补全建议 ListView         │
├──────────────────────────────┤
│ 搜索输入框 / Search 按钮      │
├──────────────────────────────┤
│ Search / Word Bank / Import  │
│ / Sync 底部导航               │
└──────────────────────────────┘
```

常见尺寸属性：

- `match_parent`：占满父容器提供的空间。
- `wrap_content`：尺寸刚好包住自身内容。
- `0dp` 配合 `layout_weight="1"`：取得父容器的剩余空间。
- `dp`：控件尺寸和间距单位。
- `sp`：文字大小单位，会跟随系统字体缩放设置。

初始状态下 `entryWebView` 是 `gone`，`welcomeHint` 可见。第一次查询后，Kotlin 动态切换它们：

```kotlin
welcomeHint.visibility = View.GONE
entryWebView.visibility = View.VISIBLE
entryActionBar.visibility = View.VISIBLE
entryTitle.text = word
```

因此 XML 主要定义初始结构和样式，Kotlin 再根据运行状态更新文字、可见性和数据。

## 6. 一个底部图标如何变成可点击页面入口

以 Word Bank 为例，完整链路包含四层。

### 6.1 drawable XML 负责画图

[`ic_nav_wordbank.xml`](../android/app/src/main/res/drawable/ic_nav_wordbank.xml) 是一个 `VectorDrawable`：

```xml
<vector
    android:width="24dp"
    android:height="24dp"
    android:viewportWidth="24"
    android:viewportHeight="24">
    <path
        android:fillColor="@android:color/black"
        android:pathData="..." />
</vector>
```

它的概念接近 SVG：

- `width` 和 `height` 是最终显示尺寸。
- `viewportWidth` 和 `viewportHeight` 是图形内部坐标系。
- `pathData` 描述线条和形状。
- `fillColor` 是原始填充颜色。

这个文件只描述图形，没有任何点击逻辑。

### 6.2 menu XML 给图标添加 ID 和标题

[`bottom_nav.xml`](../android/app/src/main/res/menu/bottom_nav.xml) 把 ID、图标和文字组合成一个菜单项：

```xml
<item
    android:id="@+id/nav_wordbank"
    android:icon="@drawable/ic_nav_wordbank"
    android:title="Word Bank" />
```

### 6.3 layout XML 把 menu 放进页面

主页面中的 `BottomNavigationView` 使用：

```xml
<com.google.android.material.bottomnavigation.BottomNavigationView
    android:id="@+id/bottomNav"
    app:menu="@menu/bottom_nav"
    app:labelVisibilityMode="labeled" />
```

`BottomNavigationView` 来自 Material Components。它会读取 `bottom_nav.xml` 并创建四个导航项目。Material 主题还可能根据选中状态对图标进行 tint，所以 VectorDrawable 中的原始黑色不一定是最终显示颜色。

### 6.4 Kotlin 处理点击后的动作

`MainActivity.setupBottomNav()` 根据被点击项目的 ID 分发动作：

```kotlin
bottomNav.setOnItemSelectedListener { item ->
    when (item.itemId) {
        R.id.nav_search -> {
            searchInput.requestFocus()
            true
        }
        R.id.nav_wordbank -> {
            WordBankActivity.start(this)
            true
        }
        R.id.nav_import -> {
            SetupActivity.start(this)
            true
        }
        R.id.nav_sync -> {
            SyncSettingsActivity.start(this)
            true
        }
        else -> false
    }
}
```

完整关系是：

```text
ic_nav_wordbank.xml 画书签
        ↓
bottom_nav.xml 将图标绑定到 nav_wordbank
        ↓
activity_main.xml 加载 bottom_nav
        ↓
用户点击菜单项
        ↓
Kotlin 收到 R.id.nav_wordbank
        ↓
启动 WordBankActivity
```

Kotlin 不需要理解 `pathData`，它只关心菜单项的资源 ID。

## 7. 搜索按钮如何调用 Go 并显示结果

`setupSearch()` 给按钮注册事件：

```kotlin
searchButton.setOnClickListener { submitSearch() }
```

搜索流程如下：

```text
用户点击 Search 或键盘搜索键
  → submitSearch()
  → 从 searchInput.text 读取单词
  → lookupAndRender(word)
  → 后台线程调用 Mobile.queryEntry(word)
  → Go 查询 MDX 并返回 HTML 片段
  → Kotlin 用 buildEntryPage() 加入 CSS 和 HTML 外壳
  → 主线程调用 entryWebView.loadDataWithBaseURL()
  → WebView 显示释义
```

Kotlin 中的 `mobile.Mobile` 来自 Go `mobile` 包。Gradle 在构建 Android 应用之前运行 `gomobile bind`，生成 `mobile.aar`：

- Android 构建配置：[`android/app/build.gradle.kts`](../android/app/build.gradle.kts)
- Go 导出接口：[`mobile/mobile.go`](../mobile/mobile.go)

常见调用包括：

```kotlin
Mobile.init(filesDir.absolutePath, cacheDir.absolutePath)
Mobile.complete(prefix, 10)
Mobile.queryEntry(word)
Mobile.getCSS()
Mobile.getFile(filename)
Mobile.wordbankAdd(word)
```

耗时的查询和文件读取放在后台 `Thread` 中；更新 Android View 则通过 `runOnUiThread` 返回主线程。Android View 不能安全地从普通后台线程直接修改。

## 8. 为什么使用 WebView，以及它如何回调 Kotlin

搜索框、导航栏和单词本按钮都是原生 Android View，但 MDX 释义保留为 HTML。这样可以继续使用词典自带的复杂 CSS、图片、交叉引用和发音图标，而不必把整个 HTML 渲染器重写成原生控件。

当前页面结构是：

```text
原生 Android 页面外壳
├── EditText
├── Button
├── ListView
├── BottomNavigationView
└── WebView
    └── Go 返回的词典 HTML + 字典 CSS
```

`setupWebView()` 主要负责三类桥接。

### 8.1 JavaScript 调用 Kotlin

`addJavascriptInterface(..., "Ondict")` 向页面暴露：

```text
Ondict.playAudio(filename)
Ondict.navigateTo(word)
```

词条 HTML 中的 JavaScript 点击监听器可以调用这些方法，让发音和交叉引用回到 Kotlin 处理。

### 8.2 Kotlin 拦截特殊链接

`WebViewClient.shouldOverrideUrlLoading()` 处理：

- `entry://word`：查询另一个词条。
- `sound://file.mp3`：读取 MDD 音频并播放。
- `/import`：打开导入页。
- `/sync`：打开同步设置页。

### 8.3 Kotlin 为 WebView 提供资源

`shouldInterceptRequest()` 使用 `Mobile.getFile(path)` 从 Go/MDD 读取图片、CSS 或音频，再以 `WebResourceResponse` 返回给 WebView，不需要访问真实网络服务器。

## 9. 为什么其他页面没有 layout XML

目前只有主页面使用 `res/layout/activity_main.xml`。以下页面直接在 Kotlin 中构造 View：

- [`SetupActivity.kt`](../android/app/src/main/java/com/ondict/app/SetupActivity.kt)
- [`WordBankActivity.kt`](../android/app/src/main/java/com/ondict/app/WordBankActivity.kt)
- [`SyncSettingsActivity.kt`](../android/app/src/main/java/com/ondict/app/SyncSettingsActivity.kt)

例如：

```kotlin
val root = LinearLayout(this).apply {
    orientation = LinearLayout.VERTICAL
}

root.addView(TextView(this).apply {
    text = "Dictionaries"
})

root.addView(Button(this).apply {
    text = "Done"
    setOnClickListener { finish() }
})

setContentView(root)
```

XML 和 Kotlin 两种写法最终都会生成 View 树：

| XML 布局 | Kotlin 动态布局 |
|---|---|
| 结构更直观 | 页面和行为集中在一个文件 |
| Android Studio 预览更方便 | 动态列表和条件内容更直接 |
| 静态页面更容易维护 | 容易混合尺寸、样式和业务逻辑 |
| 通过 `findViewById` 绑定 | 创建对象时即可保存引用和监听器 |

Setup 页的 `↑`、`↓`、`✕` 当前只是 Button 的文字，不是 `drawable` XML 图标。

## 10. 应用启动图标和页面图标不是一回事

底部导航图标位于 `res/drawable`；桌面上的应用图标位于 `res/mipmap-*`。

Manifest 中引用：

```xml
android:icon="@mipmap/ic_launcher"
android:roundIcon="@mipmap/ic_launcher_round"
```

[`mipmap-anydpi/ic_launcher.xml`](../android/app/src/main/res/mipmap-anydpi/ic_launcher.xml) 是 adaptive icon，将背景、前景和单色层组合起来：

```xml
<adaptive-icon>
    <background android:drawable="@drawable/ic_launcher_background" />
    <foreground android:drawable="@drawable/ic_launcher_foreground" />
    <monochrome android:drawable="@drawable/ic_launcher_foreground" />
</adaptive-icon>
```

Android 启动器可以根据设备风格将它裁剪成圆形、方形或圆角方形。这个过程由系统和 Manifest 完成，Kotlin 不参与。

此外，词典正文中的扬声器等图标通常来自 MDX CSS 内嵌的 icon font，它们属于 WebView/HTML 世界，也不是 Android `drawable`。

## 11. 主题、深色模式和系统栏

Manifest 为整个应用指定 `@style/Theme.Ondict`。主题定义在：

- [`res/values/themes.xml`](../android/app/src/main/res/values/themes.xml)：普通主题资源。
- [`res/values-night/themes.xml`](../android/app/src/main/res/values-night/themes.xml)：深色模式替代资源。

主题继承 `Theme.Material3.DayNight.NoActionBar`，因此 Material 控件会获得统一的颜色和样式，并随系统明暗模式变化。

所有主要 Activity 继承 [`SystemBarsAwareActivity.kt`](../android/app/src/main/java/com/ondict/app/SystemBarsAwareActivity.kt)。这个基类重写三个 `setContentView` 版本，在页面加载后统一处理状态栏、导航栏、屏幕刘海和必要时的软键盘 inset，避免控件被系统区域遮挡。

这也说明无论页面使用 XML：

```kotlin
setContentView(R.layout.activity_main)
```

还是 Kotlin View：

```kotlin
setContentView(scroll)
```

都会经过同一个系统栏适配逻辑。

## 12. 常见修改应该改哪些文件

### 增加一个底部导航入口

通常需要同时修改：

1. 在 `res/drawable` 添加 VectorDrawable 图标。
2. 在 `res/menu/bottom_nav.xml` 添加带唯一 ID 的 `<item>`。
3. 在 `MainActivity.setupBottomNav()` 增加 `when` 分支。
4. 如果打开新 Activity，在 Kotlin 中创建它。
5. 在 `AndroidManifest.xml` 注册新 Activity。

缺少第 3 步时图标可能显示但点击无有效动作；缺少第 5 步时启动 Activity 会失败。

### 在主页面增加一个按钮

通常需要：

1. 在 `activity_main.xml` 添加 Button 和 `@+id/...`。
2. 在 `MainActivity` 声明对应字段。
3. 在 `setContentView` 之后调用 `findViewById`。
4. 注册 `setOnClickListener`。

### 修改图标外观

只改形状时通常修改对应 `res/drawable/ic_*.xml` 的 `pathData`。如果问题是选中/未选中颜色，需要同时检查 Material 主题和 `BottomNavigationView` 的 icon tint，而不只是 `fillColor`。

### 修改页面初始布局

修改 `activity_main.xml`。如果布局变化涉及显示/隐藏、运行时文字或点击行为，还要检查 `MainActivity` 中是否依赖原有 ID 和 View 层次。

## 13. 推荐的源码阅读顺序

第一次复习时，建议按下面顺序阅读：

1. [`AndroidManifest.xml`](../android/app/src/main/AndroidManifest.xml)：应用有哪些页面，首页是谁。
2. [`activity_main.xml`](../android/app/src/main/res/layout/activity_main.xml)：先形成主页面的视觉结构。
3. [`bottom_nav.xml`](../android/app/src/main/res/menu/bottom_nav.xml)：理解菜单 ID、标题和图标绑定。
4. [`ic_nav_search.xml`](../android/app/src/main/res/drawable/ic_nav_search.xml)：看一个 VectorDrawable 的结构。
5. [`MainActivity.onCreate()`](../android/app/src/main/java/com/ondict/app/MainActivity.kt)：看 XML 如何被加载并绑定。
6. `setupBottomNav()`：看资源 ID 如何变成页面跳转。
7. `setupSearch()` 和 `lookupAndRender()`：看用户输入如何进入 Go 查询。
8. `setupWebView()`：理解 HTML、JavaScript、Kotlin 和 Go 的桥接。
9. [`WordBankActivity.kt`](../android/app/src/main/java/com/ondict/app/WordBankActivity.kt)：对比纯 Kotlin 构造页面。
10. [`mobile/mobile.go`](../mobile/mobile.go)：最后看 Android UI 后面的 Go 接口。

## 14. 当前实现与旧文档的区别

仓库中部分旧说明仍把 Android 描述为“启动本地 HTTP server，再把完整 Web 页面放进 WebView”。当前 `MainActivity` 的实际实现已经是：

- Kotlin 原生页面外壳。
- WebView 只显示词条内容。
- Kotlin 通过 `mobile.Mobile` 直接调用 Go 查询。
- 使用 `loadDataWithBaseURL()` 加载 HTML。
- 使用 WebViewClient 和 JavaScript interface 处理资源、发音和词条跳转。
- 查询和渲染路径不需要本地 HTTP server。

阅读 Android UI 架构时，应优先以 `MainActivity.kt`、`mobile/mobile.go` 和实际 Gradle 配置为准。

## 15. 最短记忆版本

如果只记住五点：

1. `AndroidManifest.xml` 决定应用入口和页面注册。
2. `activity_main.xml` 决定主页面初始 View 树。
3. Android 编译器通过 `R` 把 XML 资源暴露给 Kotlin。
4. drawable 只负责画图；菜单 ID 和 Kotlin listener 才决定点击行为。
5. Ondict 的原生控件由 Kotlin 控制，复杂 MDX 释义由 Go 生成 HTML 后交给 WebView 显示。
