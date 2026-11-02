package org.braid.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.braid.app.Names
import org.braid.app.Units

/** A rounded card with a hairline border, the building block of every screen. */
@Composable
fun Card(modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    val p = LocalPalette.current
    Box(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(24.dp))
            .background(p.card)
            .border(1.dp, p.cardBorder, RoundedCornerShape(24.dp))
            .padding(18.dp),
    ) { content() }
}

/** A small heading above a group of cards. */
@Composable
fun SectionTitle(text: String) {
    Text(
        text,
        color = LocalPalette.current.muted,
        style = MaterialTheme.typography.labelLarge,
        modifier = Modifier.fillMaxWidth().padding(start = 6.dp, end = 6.dp, top = 6.dp, bottom = 2.dp),
    )
}

@Composable
fun Metric(label: String, value: String, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    Column(modifier) {
        Text(label, color = p.muted, fontSize = 12.sp)
        Text(value, color = MaterialTheme.colorScheme.onSurface, fontSize = 15.sp, fontWeight = FontWeight.Medium)
    }
}

/** A round tinted badge with an icon, used at the start of list rows. */
@Composable
fun IconBadge(iconRes: Int, tint: Color, size: Int = 42) {
    Box(
        Modifier.size(size.dp).clip(CircleShape).background(tint.copy(alpha = 0.16f)),
        contentAlignment = Alignment.Center,
    ) {
        Icon(painterResource(iconRes), contentDescription = null, tint = tint, modifier = Modifier.size((size / 2 + 1).dp))
    }
}

/** One line of a checklist: a green tick or a grey circle, then the text. */
@Composable
fun CheckRow(ok: Boolean, text: String) {
    val p = LocalPalette.current
    Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 3.dp)) {
        Icon(
            painterResource(if (ok) org.braid.app.R.drawable.ic_check_circle else org.braid.app.R.drawable.ic_cancel),
            contentDescription = null,
            tint = if (ok) p.good else p.muted,
            modifier = Modifier.size(20.dp),
        )
        Spacer(Modifier.width(10.dp))
        Text(text, color = if (ok) MaterialTheme.colorScheme.onSurface else p.muted, fontSize = 14.sp)
    }
}

@Composable
fun rateText(bytesPerSec: Double): String = Units.rate(LocalContext.current, bytesPerSec)

@Composable
fun sizeText(bytes: Double): String = Units.size(LocalContext.current, bytes)

@Composable
fun networkName(name: String): String = Names.network(LocalContext.current, name)

@Composable
fun statusName(status: String): String = Names.status(LocalContext.current, status)

/** Western digits as the current language writes them (Persian digits in Persian). */
@Composable
fun String.localDigits(): String {
    val lang = androidx.compose.ui.platform.LocalConfiguration.current.locales[0].language
    if (lang != "fa") return this
    return map { if (it in '0'..'9') '۰' + (it - '0') else it }.joinToString("")
}
