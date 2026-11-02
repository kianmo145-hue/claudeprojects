package org.peyvand.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.peyvand.app.BuildConfig
import org.peyvand.app.R

@Composable
fun SettingsScreen(
    lang: String,
    theme: String,
    evenSplit: Boolean,
    onLang: (String) -> Unit,
    onTheme: (String) -> Unit,
    onEvenSplit: (Boolean) -> Unit,
) {
    val p = LocalPalette.current
    Column(verticalArrangement = Arrangement.spacedBy(14.dp), modifier = Modifier.fillMaxWidth()) {
        Text(
            stringResource(R.string.settings),
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onBackground,
            modifier = Modifier.fillMaxWidth(),
        )

        SectionTitle(stringResource(R.string.language))
        Card {
            Choices(
                listOf("system" to stringResource(R.string.follow_system), "fa" to "فارسی", "en" to "English"),
                lang, onLang,
            )
        }

        SectionTitle(stringResource(R.string.theme))
        Card {
            Choices(
                listOf(
                    "system" to stringResource(R.string.follow_system),
                    "light" to stringResource(R.string.theme_light),
                    "dark" to stringResource(R.string.theme_dark),
                ),
                theme, onTheme,
            )
        }

        SectionTitle(stringResource(R.string.merge_title))
        Card {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(stringResource(R.string.even_split), fontWeight = FontWeight.SemiBold, color = MaterialTheme.colorScheme.onSurface)
                    Text(stringResource(R.string.even_split_hint), color = p.muted, fontSize = 13.sp, lineHeight = 19.sp)
                }
                Spacer(Modifier.width(12.dp))
                Switch(checked = evenSplit, onCheckedChange = onEvenSplit, colors = SwitchDefaults.colors(checkedTrackColor = p.brand))
            }
        }

        SectionTitle(stringResource(R.string.about))
        Card {
            Column {
                Text(stringResource(R.string.app_name) + "  " + BuildConfig.VERSION_NAME.localDigits(), fontWeight = FontWeight.SemiBold, color = MaterialTheme.colorScheme.onSurface)
                Spacer(Modifier.height(6.dp))
                Text(stringResource(R.string.about_body), color = p.muted, fontSize = 13.sp, lineHeight = 20.sp)
            }
        }
    }
}

@Composable
private fun Choices(options: List<Pair<String, String>>, selected: String, onSelect: (String) -> Unit) {
    Column(Modifier.selectableGroup()) {
        for ((value, label) in options) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(46.dp)
                    .selectable(selected = value == selected, onClick = { onSelect(value) }, role = Role.RadioButton),
            ) {
                RadioButton(selected = value == selected, onClick = null)
                Spacer(Modifier.width(10.dp))
                Text(label, color = MaterialTheme.colorScheme.onSurface, fontSize = 15.sp)
            }
        }
    }
}
