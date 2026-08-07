package com.ondict.app

import android.view.View
import android.view.ViewGroup
import androidx.appcompat.app.AppCompatActivity
import androidx.core.graphics.Insets
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import kotlin.math.max

/**
 * Keeps an activity's entire content area outside system bars and display
 * cutouts.
 *
 * Android 16 ignores windowOptOutEdgeToEdgeEnforcement for apps targeting API
 * 36, so the shared theme's opt-out alone cannot prevent content overlap. This
 * base class reproduces the non-edge-to-edge content area with root padding.
 */
abstract class SystemBarsAwareActivity : AppCompatActivity() {

    /** Override on screens whose controls must remain above the soft keyboard. */
    protected open val avoidImeOverlap: Boolean = false

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
            val imeBottom = if (avoidImeOverlap) {
                windowInsets.getInsets(WindowInsetsCompat.Type.ime()).bottom
            } else {
                0
            }
            view.setPadding(
                initial.left + protectedInsets.left,
                initial.top + protectedInsets.top,
                initial.right + protectedInsets.right,
                initial.bottom + max(protectedInsets.bottom, imeBottom)
            )

            // Remove the insets handled above before dispatching to children,
            // preventing inset-aware controls from applying duplicate padding.
            // Screens that do not avoid the IME here still receive its inset.
            val childInsets = WindowInsetsCompat.Builder(windowInsets)
                .setInsets(protectedInsetTypes, Insets.NONE)
                .setInsetsIgnoringVisibility(protectedInsetTypes, Insets.NONE)
            if (avoidImeOverlap) {
                childInsets.setInsets(WindowInsetsCompat.Type.ime(), Insets.NONE)
            }
            childInsets.build()
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
