package com.ondict.app

import android.view.View
import android.view.ViewGroup
import androidx.appcompat.app.AppCompatActivity
import androidx.core.graphics.Insets
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat

/**
 * Keeps an activity's entire content area outside system bars and display
 * cutouts.
 *
 * Android 16 ignores windowOptOutEdgeToEdgeEnforcement for apps targeting API
 * 36, so the shared theme's opt-out alone cannot prevent content overlap. This
 * base class reproduces the non-edge-to-edge content area with root padding.
 */
abstract class SystemBarsAwareActivity : AppCompatActivity() {

    private var initialContentPadding: ContentPadding? = null

    override fun setContentView(layoutResID: Int) {
        super.setContentView(layoutResID)
        applySystemBarInsets()
    }

    override fun setContentView(view: View) {
        super.setContentView(view)
        applySystemBarInsets()
    }

    override fun setContentView(view: View, params: ViewGroup.LayoutParams) {
        super.setContentView(view, params)
        applySystemBarInsets()
    }

    private fun applySystemBarInsets() {
        val content = findViewById<View>(android.R.id.content)
        val protectedInsetTypes = WindowInsetsCompat.Type.systemBars() or
            WindowInsetsCompat.Type.displayCutout()
        val initial = initialContentPadding ?: ContentPadding(
            left = content.paddingLeft,
            top = content.paddingTop,
            right = content.paddingRight,
            bottom = content.paddingBottom
        ).also { initialContentPadding = it }

        ViewCompat.setOnApplyWindowInsetsListener(content) { view, windowInsets ->
            val protectedInsets = windowInsets.getInsets(protectedInsetTypes)
            view.setPadding(
                initial.left + protectedInsets.left,
                initial.top + protectedInsets.top,
                initial.right + protectedInsets.right,
                initial.bottom + protectedInsets.bottom
            )

            // The activity content has handled these insets. Remove only those
            // types before dispatching to children so IME insets still support
            // adjustResize while inset-aware controls avoid double padding.
            WindowInsetsCompat.Builder(windowInsets)
                .setInsets(protectedInsetTypes, Insets.NONE)
                .setInsetsIgnoringVisibility(protectedInsetTypes, Insets.NONE)
                .build()
        }
        ViewCompat.requestApplyInsets(content)
    }

    private data class ContentPadding(
        val left: Int,
        val top: Int,
        val right: Int,
        val bottom: Int
    )
}
