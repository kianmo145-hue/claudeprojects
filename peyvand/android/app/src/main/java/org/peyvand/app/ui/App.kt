package org.peyvand.app.ui

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import org.peyvand.app.R

enum class Tab { MERGE, BOOSTER, SETTINGS }

/** The app's frame: a soft gradient, the current screen, and a bottom bar with three tabs. */
@Composable
fun App(tab: Tab, onTab: (Tab) -> Unit, content: @Composable (Tab) -> Unit) {
    val p = LocalPalette.current
    Column(
        Modifier
            .fillMaxSize()
            .background(Brush.verticalGradient(listOf(p.backgroundTop, p.backgroundBottom))),
    ) {
        Box(Modifier.weight(1f)) {
            AnimatedContent(
                targetState = tab,
                transitionSpec = { fadeIn(tween(250)) togetherWith fadeOut(tween(150)) },
                label = "tab",
            ) { t ->
                Column(
                    Modifier
                        .fillMaxSize()
                        .windowInsetsPadding(WindowInsets.safeDrawing.only(WindowInsetsSides.Top + WindowInsetsSides.Horizontal))
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 18.dp, vertical = 16.dp),
                ) {
                    content(t)
                    Spacer(Modifier.height(12.dp))
                }
            }
        }
        NavigationBar(containerColor = p.card, tonalElevation = 0.dp) {
            val items = listOf(
                Triple(Tab.MERGE, R.drawable.ic_merge, R.string.tab_merge),
                Triple(Tab.BOOSTER, R.drawable.ic_phone_link, R.string.tab_booster),
                Triple(Tab.SETTINGS, R.drawable.ic_settings, R.string.tab_settings),
            )
            for ((t, icon, label) in items) {
                NavigationBarItem(
                    selected = tab == t,
                    onClick = { onTab(t) },
                    icon = { Icon(painterResource(icon), contentDescription = null) },
                    label = { Text(stringResource(label)) },
                    colors = NavigationBarItemDefaults.colors(indicatorColor = p.brand.copy(alpha = 0.2f), selectedIconColor = p.brand, selectedTextColor = p.brand),
                )
            }
        }
    }
}
