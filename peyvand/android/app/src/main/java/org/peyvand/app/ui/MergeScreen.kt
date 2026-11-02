package org.peyvand.app.ui

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.peyvand.app.Engine
import org.peyvand.app.R
import org.peyvand.app.Stats

/** Merge mode: Wi-Fi and mobile data together on this phone. */
@Composable
fun MergeScreen(
    phase: Engine.Phase,
    stats: List<Stats>,
    history: Map<String, List<Float>>,
    onToggle: () -> Unit,
) {
    Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
        Header(phase, stats.size)
        Spacer(Modifier.height(18.dp))
        PowerButton(
            on = phase == Engine.Phase.ON,
            starting = phase == Engine.Phase.STARTING,
            onClick = onToggle,
            size = 176.dp,
            label = stringResource(if (phase == Engine.Phase.OFF) R.string.turn_on else R.string.turn_off),
        )
        Spacer(Modifier.height(16.dp))
        CombinedSpeed(phase, stats)
        Spacer(Modifier.height(18.dp))

        AnimatedVisibility(
            visible = phase == Engine.Phase.ON,
            enter = fadeIn(tween(500)) + expandVertically(tween(500)),
            exit = fadeOut(tween(300)) + shrinkVertically(tween(300)),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                for ((i, s) in stats.withIndex()) NetworkCard(s, stats, i)
                if (stats.isEmpty()) {
                    Card { Text(stringResource(R.string.waiting_networks), color = LocalPalette.current.muted, fontSize = 14.sp) }
                }
                ChartCard(stats, history)
            }
        }
        AnimatedVisibility(visible = phase == Engine.Phase.OFF, enter = fadeIn(tween(500)), exit = fadeOut(tween(200))) {
            IntroCard()
        }
    }
}

@Composable
private fun Header(phase: Engine.Phase, networks: Int) {
    val p = LocalPalette.current
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            stringResource(R.string.merge_title),
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onBackground,
        )
        AnimatedContent(
            targetState = phase to networks,
            transitionSpec = { (fadeIn(tween(300)) + slideInVertically { it / 2 }) togetherWith fadeOut(tween(200)) },
            label = "status",
        ) { (ph, n) ->
            val (text, color) = when (ph) {
                Engine.Phase.OFF -> stringResource(R.string.merge_off) to p.muted
                Engine.Phase.STARTING -> stringResource(R.string.connecting) to p.warn
                Engine.Phase.ON -> when (n) {
                    0 -> stringResource(R.string.waiting_short) to p.warn
                    1 -> stringResource(R.string.using_one) to p.warn
                    else -> stringResource(R.string.merging_n, n) to p.good
                }
            }
            Text(text, color = color, fontSize = 15.sp, modifier = Modifier.padding(top = 4.dp))
        }
    }
}

@Composable
private fun CombinedSpeed(phase: Engine.Phase, stats: List<Stats>) {
    val p = LocalPalette.current
    val down by animateFloatAsState(stats.sumOf { it.rateIn }.toFloat(), tween(800), label = "down")
    val up by animateFloatAsState(stats.sumOf { it.rateOut }.toFloat(), tween(800), label = "up")
    val alpha by animateFloatAsState(if (phase == Engine.Phase.ON) 1f else 0.35f, tween(500), label = "a")
    Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
        Text(
            rateText((down + up).toDouble()),
            fontSize = 38.sp,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onBackground.copy(alpha = alpha),
        )
        Text(stringResource(R.string.combined), color = p.muted.copy(alpha = alpha), fontSize = 13.sp)
        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(24.dp)) {
            SpeedLabel("↓", down, p.wifi.copy(alpha = alpha))
            SpeedLabel("↑", up, p.cell.copy(alpha = alpha))
        }
    }
}

@Composable
private fun SpeedLabel(arrow: String, value: Float, color: Color) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(arrow, color = color, fontSize = 18.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.width(6.dp))
        Text(rateText(value.toDouble()), color = MaterialTheme.colorScheme.onBackground, fontSize = 15.sp)
    }
}

fun colorFor(name: String, p: Palette): Color = if (name.startsWith("Wi-Fi")) p.wifi else p.cell

@Composable
private fun ChartCard(stats: List<Stats>, history: Map<String, List<Float>>) {
    val p = LocalPalette.current
    Card {
        Column {
            Text(stringResource(R.string.last_minute), color = p.muted, fontSize = 13.sp)
            Spacer(Modifier.height(10.dp))
            LiveChart(
                series = stats.map { colorFor(it.name, p) to (history[it.name] ?: emptyList()) },
                modifier = Modifier.fillMaxWidth().height(130.dp).clip(RoundedCornerShape(10.dp)),
            )
        }
    }
}

@Composable
private fun NetworkCard(s: Stats, all: List<Stats>, index: Int) {
    val p = LocalPalette.current
    val color = colorFor(s.name, p)
    val total = all.sumOf { it.rateIn + it.rateOut }
    val share by animateFloatAsState(
        if (total > 0) ((s.rateIn + s.rateOut) / total).toFloat() else 0f,
        tween(900, easing = FastOutSlowInEasing), label = "share",
    )
    val statusColor by animateColorAsState(
        when (s.status) {
            "up" -> p.good
            "checking" -> p.warn
            else -> p.bad
        },
        tween(500), label = "status",
    )
    val pulse = rememberInfiniteTransition(label = "pulse")
    val dot by pulse.animateFloat(0.4f, 1f, infiniteRepeatable(tween(900 + index * 150), RepeatMode.Reverse), label = "dot")
    Card {
        Column {
            Row(verticalAlignment = Alignment.CenterVertically) {
                IconBadge(if (s.name.startsWith("Wi-Fi")) R.drawable.ic_wifi else R.drawable.ic_cell, color)
                Spacer(Modifier.width(12.dp))
                Column(Modifier.weight(1f)) {
                    Text(networkName(s.name), fontSize = 17.sp, fontWeight = FontWeight.SemiBold, color = MaterialTheme.colorScheme.onSurface)
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Canvas(Modifier.size(8.dp)) { drawCircle(statusColor.copy(alpha = if (s.status == "up") dot else 1f)) }
                        Spacer(Modifier.width(6.dp))
                        Text(statusName(s.status), color = statusColor, fontSize = 13.sp)
                        Text("  ·  " + stringResource(R.string.open_flows, s.active), color = p.muted, fontSize = 13.sp)
                    }
                }
                Text(
                    stringResource(R.string.percent, (share * 100).toInt()),
                    fontSize = 22.sp, fontWeight = FontWeight.Bold, color = color, textAlign = TextAlign.End,
                )
            }
            Spacer(Modifier.height(14.dp))
            // Share of the combined traffic this network is carrying right now.
            Canvas(Modifier.fillMaxWidth().height(8.dp)) {
                val r = CornerRadius(size.height / 2)
                drawRoundRect(p.muted.copy(alpha = 0.18f), cornerRadius = r)
                val w = size.width * share
                // In right-to-left layouts the bar grows from the right.
                val left = if (layoutDirection == androidx.compose.ui.unit.LayoutDirection.Rtl) size.width - w else 0f
                drawRoundRect(
                    Brush.horizontalGradient(listOf(color.copy(alpha = 0.6f), color)),
                    topLeft = androidx.compose.ui.geometry.Offset(left, 0f),
                    size = size.copy(width = w), cornerRadius = r,
                )
            }
            Spacer(Modifier.height(12.dp))
            Row {
                Metric(stringResource(R.string.download), rateText(s.rateIn), Modifier.weight(1f))
                Metric(stringResource(R.string.upload), rateText(s.rateOut), Modifier.weight(1f))
                Metric(stringResource(R.string.moved), sizeText((s.bytesIn + s.bytesOut).toDouble()), Modifier.weight(1f))
            }
        }
    }
}

@Composable
private fun IntroCard() {
    val p = LocalPalette.current
    Card {
        Column {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(painterResource(R.drawable.ic_wifi), null, tint = p.wifi, modifier = Modifier.size(20.dp))
                Text("  +  ", color = p.muted)
                Icon(painterResource(R.drawable.ic_cell), null, tint = p.cell, modifier = Modifier.size(20.dp))
                Spacer(Modifier.width(10.dp))
                Text(stringResource(R.string.intro_title), fontWeight = FontWeight.SemiBold, color = MaterialTheme.colorScheme.onSurface)
            }
            Spacer(Modifier.height(8.dp))
            Text(stringResource(R.string.intro_body), color = p.muted, fontSize = 14.sp, lineHeight = 22.sp)
        }
    }
}
