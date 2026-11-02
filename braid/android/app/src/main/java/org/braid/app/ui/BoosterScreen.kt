package org.braid.app.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.platform.LocalLayoutDirection
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.braid.app.Booster
import org.braid.app.Prefs
import org.braid.app.R

/** Booster mode: share this phone's mobile data with a paired computer on the same Wi-Fi. */
@Composable
fun BoosterScreen(
    phase: Booster.Phase,
    links: Booster.Links,
    clients: List<Booster.Client>,
    pcs: List<Prefs.Pc>,
    history: List<Float>,
    onToggle: () -> Unit,
    onForget: (String) -> Unit,
) {
    val p = LocalPalette.current
    val on = phase == Booster.Phase.ON
    val active = clients.filter { it.active > 0 || it.rateIn + it.rateOut > 0 }
    Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
        Text(
            stringResource(R.string.boost_title),
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onBackground,
        )
        val (status, color) = when {
            !on -> stringResource(R.string.boost_off) to p.muted
            !links.wifi -> stringResource(R.string.boost_need_wifi) to p.warn
            !links.cell -> stringResource(R.string.boost_need_cell) to p.warn
            active.isEmpty() -> stringResource(R.string.boost_ready) to p.good
            else -> stringResource(R.string.boost_sharing_n, active.size) to p.good
        }
        Text(status, color = color, fontSize = 15.sp, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 4.dp))
        Spacer(Modifier.height(18.dp))

        Bridge(on = on && links.wifi && links.cell, busy = active.isNotEmpty())
        Spacer(Modifier.height(14.dp))
        PowerButton(
            on = on,
            starting = false,
            onClick = onToggle,
            size = 150.dp,
            ring = listOf(p.cell, p.brand, p.wifi, p.cell),
            label = stringResource(if (on) R.string.turn_off else R.string.turn_on),
        )
        Spacer(Modifier.height(14.dp))

        val total = clients.sumOf { it.rateIn + it.rateOut }
        val shown by animateFloatAsState(total.toFloat(), tween(800), label = "total")
        val alpha by animateFloatAsState(if (on) 1f else 0.35f, tween(500), label = "a")
        Text(rateText(shown.toDouble()), fontSize = 34.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onBackground.copy(alpha = alpha))
        Text(stringResource(R.string.boost_to_computers), color = p.muted.copy(alpha = alpha), fontSize = 13.sp)
        Spacer(Modifier.height(18.dp))

        Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
            AnimatedVisibility(visible = on, enter = fadeIn(tween(400)) + expandVertically(tween(400)), exit = fadeOut(tween(200)) + shrinkVertically(tween(200))) {
                Card {
                    Column {
                        CheckRow(links.wifi, if (links.wifi) stringResource(R.string.check_wifi_ip, links.wifiIp) else stringResource(R.string.check_wifi))
                        CheckRow(links.cell, stringResource(R.string.check_cell))
                        Spacer(Modifier.height(10.dp))
                        LiveChart(
                            series = listOf(p.cell to history),
                            modifier = Modifier.fillMaxWidth().height(90.dp).clip(RoundedCornerShape(10.dp)),
                        )
                        Spacer(Modifier.height(10.dp))
                        Row {
                            Metric(stringResource(R.string.boost_used), sizeText(clients.sumOf { it.bytesIn + it.bytesOut }.toDouble()), Modifier.weight(1f))
                            Metric(stringResource(R.string.open_connections), clients.sumOf { it.active }.toString(), Modifier.weight(1f))
                        }
                    }
                }
            }

            SectionTitle(stringResource(R.string.computers))
            if (pcs.isEmpty()) {
                HowTo()
            } else {
                for (pc in pcs) {
                    ComputerRow(pc, clients.firstOrNull { it.id == pc.id }, onForget)
                }
                Text(stringResource(R.string.add_another), color = p.muted, fontSize = 13.sp, lineHeight = 20.sp, modifier = Modifier.padding(horizontal = 6.dp))
            }
            Card {
                Text(stringResource(R.string.boost_data_note), color = p.muted, fontSize = 13.sp, lineHeight = 20.sp)
            }
        }
    }
}

/** Phone on one side, computer on the other, with data dots travelling between them while on. */
@Composable
private fun Bridge(on: Boolean, busy: Boolean) {
    val p = LocalPalette.current
    val rtl = LocalLayoutDirection.current == LayoutDirection.Rtl
    val t = rememberInfiniteTransition(label = "bridge")
    val phase by t.animateFloat(0f, 1f, infiniteRepeatable(tween(if (busy) 900 else 1800, easing = LinearEasing)), label = "flow")
    Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
        IconBadge(R.drawable.ic_phone, p.cell, 58)
        Canvas(Modifier.weight(1f).height(24.dp).padding(horizontal = 10.dp)) {
            val y = size.height / 2
            drawLine(
                p.muted.copy(alpha = 0.35f), Offset(0f, y), Offset(size.width, y), strokeWidth = 3f,
                pathEffect = PathEffect.dashPathEffect(floatArrayOf(10f, 10f)),
            )
            if (on) {
                for (k in 0 until 4) {
                    var f = (phase + k / 4f) % 1f
                    if (rtl) f = 1f - f // the phone is on the right in RTL; data still flows phone -> computer
                    val c = lerp(p.cell, p.wifi, if (rtl) 1f - f else f)
                    drawCircle(c, radius = 6f, center = Offset(size.width * f, y))
                }
            }
        }
        IconBadge(R.drawable.ic_laptop, p.wifi, 58)
    }
}

private fun lerp(a: Color, b: Color, t: Float) = Color(
    a.red + (b.red - a.red) * t, a.green + (b.green - a.green) * t, a.blue + (b.blue - a.blue) * t, 1f,
)

@Composable
private fun ComputerRow(pc: Prefs.Pc, live: Booster.Client?, onForget: (String) -> Unit) {
    val p = LocalPalette.current
    var confirm by remember { mutableStateOf(false) }
    val busy = live != null && (live.active > 0 || live.rateIn + live.rateOut > 0)
    Card {
        Row(verticalAlignment = Alignment.CenterVertically) {
            IconBadge(R.drawable.ic_laptop, if (busy) p.good else p.muted)
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(pc.name, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = MaterialTheme.colorScheme.onSurface)
                val sub = when {
                    busy -> stringResource(R.string.pc_active, rateText(live!!.rateIn + live.rateOut), live.active)
                    live != null -> stringResource(R.string.pc_used, sizeText((live.bytesIn + live.bytesOut).toDouble()))
                    else -> stringResource(R.string.pc_paired)
                }
                Text(sub, color = if (busy) p.good else p.muted, fontSize = 13.sp)
            }
            IconButton(onClick = { confirm = true }) {
                Icon(painterResource(R.drawable.ic_delete), contentDescription = stringResource(R.string.forget), tint = p.muted)
            }
        }
    }
    if (confirm) {
        AlertDialog(
            onDismissRequest = { confirm = false },
            title = { Text(stringResource(R.string.forget_title, pc.name)) },
            text = { Text(stringResource(R.string.forget_body)) },
            confirmButton = { TextButton(onClick = { confirm = false; onForget(pc.id) }) { Text(stringResource(R.string.forget)) } },
            dismissButton = { TextButton(onClick = { confirm = false }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

@Composable
private fun HowTo() {
    val p = LocalPalette.current
    Card {
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(stringResource(R.string.howto_title), fontWeight = FontWeight.SemiBold, color = MaterialTheme.colorScheme.onSurface)
            val steps = listOf(R.string.howto_1, R.string.howto_2, R.string.howto_3, R.string.howto_4)
            for ((i, s) in steps.withIndex()) {
                Row(verticalAlignment = Alignment.Top) {
                    Box(
                        Modifier.size(26.dp).clip(CircleShape).background(p.brand.copy(alpha = 0.18f)),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text("${i + 1}".localDigits(), color = p.brand, fontWeight = FontWeight.Bold, fontSize = 13.sp)
                    }
                    Spacer(Modifier.width(12.dp))
                    Text(stringResource(s), color = p.muted, fontSize = 14.sp, lineHeight = 21.sp, modifier = Modifier.weight(1f))
                }
            }
        }
    }
}

/** The pairing prompt: the computer's name and the code to compare, big. */
@Composable
fun PairDialog(req: Booster.PairRequest, onAnswer: (Boolean) -> Unit) {
    val p = LocalPalette.current
    AlertDialog(
        onDismissRequest = { },
        icon = { Icon(painterResource(R.drawable.ic_laptop), contentDescription = null, tint = p.brand) },
        title = { Text(stringResource(R.string.pair_title, req.pcName), textAlign = TextAlign.Center) },
        text = {
            Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
                Text(stringResource(R.string.pair_compare), textAlign = TextAlign.Center, color = p.muted)
                Spacer(Modifier.height(16.dp))
                // Codes are compared digit by digit, so they are always shown left to right.
                androidx.compose.runtime.CompositionLocalProvider(LocalLayoutDirection provides LayoutDirection.Ltr) {
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        for ((i, ch) in req.code.withIndex()) {
                            if (i == 3) Spacer(Modifier.width(10.dp))
                            Box(
                                Modifier.size(width = 34.dp, height = 46.dp).clip(RoundedCornerShape(10.dp)).background(p.brand.copy(alpha = 0.14f)),
                                contentAlignment = Alignment.Center,
                            ) {
                                Text(ch.toString().localDigits(), fontSize = 26.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSurface)
                            }
                        }
                    }
                }
                Spacer(Modifier.height(16.dp))
                Text(stringResource(R.string.pair_hint), textAlign = TextAlign.Center, color = p.muted, fontSize = 13.sp)
            }
        },
        confirmButton = { Button(onClick = { onAnswer(true) }) { Text(stringResource(R.string.pair_approve)) } },
        dismissButton = { TextButton(onClick = { onAnswer(false) }) { Text(stringResource(R.string.pair_decline)) } },
    )
}
