package org.peyvand.app.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import org.peyvand.app.R

/** Colours that carry meaning across the app: one per network type, the brand, and states. */
@Immutable
data class Palette(
    val wifi: Color,
    val cell: Color,
    val brand: Color,
    val brand2: Color,
    val good: Color,
    val warn: Color,
    val bad: Color,
    val backgroundTop: Color,
    val backgroundBottom: Color,
    val card: Color,
    val cardBorder: Color,
    val muted: Color,
)

private val DarkPalette = Palette(
    wifi = Color(0xFF38BDF8),
    cell = Color(0xFFFB923C),
    brand = Color(0xFF34D399),
    brand2 = Color(0xFF14B8A6),
    good = Color(0xFF34D399),
    warn = Color(0xFFFBBF24),
    bad = Color(0xFFF87171),
    backgroundTop = Color(0xFF07131A),
    backgroundBottom = Color(0xFF0C1E28),
    card = Color(0xFF112630),
    cardBorder = Color(0xFF1C3A47),
    muted = Color(0xFF94A3B8),
)

private val LightPalette = Palette(
    wifi = Color(0xFF0284C7),
    cell = Color(0xFFEA580C),
    brand = Color(0xFF059669),
    brand2 = Color(0xFF0D9488),
    good = Color(0xFF059669),
    warn = Color(0xFFD97706),
    bad = Color(0xFFDC2626),
    backgroundTop = Color(0xFFF0FAF7),
    backgroundBottom = Color(0xFFE3F1EE),
    card = Color(0xFFFFFFFF),
    cardBorder = Color(0xFFD5E7E2),
    muted = Color(0xFF64748B),
)

val LocalPalette = staticCompositionLocalOf { DarkPalette }

/** Vazirmatn covers Persian and Latin, so one family serves both languages. */
val Vazirmatn = FontFamily(
    Font(R.font.vazirmatn_regular, FontWeight.Normal),
    Font(R.font.vazirmatn_medium, FontWeight.Medium),
    Font(R.font.vazirmatn_medium, FontWeight.SemiBold),
    Font(R.font.vazirmatn_bold, FontWeight.Bold),
)

private fun TextStyle.vazir() = copy(fontFamily = Vazirmatn)

private val AppTypography = Typography().run {
    copy(
        displayLarge = displayLarge.vazir(), displayMedium = displayMedium.vazir(), displaySmall = displaySmall.vazir(),
        headlineLarge = headlineLarge.vazir(), headlineMedium = headlineMedium.vazir(), headlineSmall = headlineSmall.vazir(),
        titleLarge = titleLarge.vazir(), titleMedium = titleMedium.vazir(), titleSmall = titleSmall.vazir(),
        bodyLarge = bodyLarge.vazir(), bodyMedium = bodyMedium.vazir(), bodySmall = bodySmall.vazir(),
        labelLarge = labelLarge.vazir(), labelMedium = labelMedium.vazir(), labelSmall = labelSmall.vazir(),
    )
}

@Composable
fun PeyvandTheme(dark: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    val palette = if (dark) DarkPalette else LightPalette
    val scheme = if (dark) {
        darkColorScheme(
            primary = palette.brand,
            onPrimary = Color(0xFF032A20),
            secondary = palette.brand2,
            background = palette.backgroundTop,
            surface = palette.card,
            surfaceContainer = palette.card,
            onSurface = Color(0xFFE2E8F0),
            onBackground = Color(0xFFE2E8F0),
        )
    } else {
        lightColorScheme(
            primary = palette.brand,
            onPrimary = Color.White,
            secondary = palette.brand2,
            background = palette.backgroundTop,
            surface = palette.card,
            surfaceContainer = palette.card,
            onSurface = Color(0xFF0F172A),
            onBackground = Color(0xFF0F172A),
        )
    }
    CompositionLocalProvider(LocalPalette provides palette) {
        MaterialTheme(colorScheme = scheme, typography = AppTypography, content = content)
    }
}
