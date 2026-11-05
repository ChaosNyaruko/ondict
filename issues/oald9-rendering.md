# oald9 Rendering — Deferred Issues

## Background

oald9 (Oxford Advanced Learner's Dictionary 9) uses a different raw HTML dialect
from LDOCE5++. Its entries are not processed by the current `defaultHandlers`
pipeline beyond `EntryHandler` (which handles `entry://`). The following patterns
are unhandled and need dedicated handlers.

---

## 1. `snd://` audio scheme + custom `<audio-*>` elements

Raw form (headword pronunciation):
```html
<a href="snd://apple__gb_1.spx"><audio-gb>🔊</audio-gb></a>
<a href="snd://apple__us_1.spx"><audio-us>🔊</audio-us></a>
```

Raw form (example sentence audio):
```html
<a href="snd://_apple__gbs_2.spx"><audio-gbs-liju>🔊</audio-gbs-liju></a>
<a href="snd://_apple__ams_2.spx"><audio-ams-liju>🔊</audio-ams-liju></a>
<a href="snd://_apple__brs_2.spx"><audio-brs-liju>🔊</audio-brs-liju></a>
<a href="snd://_apple__uss_2.spx"><audio-uss-liju>🔊</audio-uss-liju></a>
```

The `snd://` scheme is not matched by `SoundHandler` (which only checks `sound://`).
The inner `<audio-gb>`, `<audio-us>`, `<audio-gbs-liju>` etc. are custom elements
that pass through as unknown tags — the emoji text content renders but clicking does
nothing.

Audio files are `.spx` (Speex format). MDD serving may need to handle `.spx` content
type (`audio/ogg` or `audio/x-speex`), or files may need transcoding. Verify what
the MDD actually contains before implementing.

**Fix:** Add a `SndHandler` that matches `href="snd://"` and applies the same
`data-audio-src` attribute approach used for `sound://`.

---

## 2. `xhtml:a` decorative word-links

Raw form:
```html
<xhtml:a href="d:round">round</xhtml:a>
<xhtml:a href="x:apples">apples</xhtml:a>
<xhtml:a href="help:keyword">...</xhtml:a>
<xhtml:a href="helpp:n">noun</xhtml:a>
<xhtml:a href="helpg:am">NAmE</xhtml:a>
```

Schemes seen: `d:`, `x:`, `xi:`, `xp:`, `xid:`, `help:`, `helpp:`, `helpg:`,
`helpgr:`, `helpxr:`, `helpr:`, `helpil:`, `addexample:`, `addpv:`, `addid:`.

The Go HTML parser treats `xhtml:a` as an unknown element (namespace prefix is
part of the tag name). It renders but the `href` values are meaningless in a web
context — `d:round` is an internal dict app URI, not a real URL.

`d:word` and `x:word` are definition/example word links (analogous to `entry://`
in LDOCE5++ but lower-fidelity — they don't always map to a real headword).
`help:*` are app-internal help links. `addexample:*`, `addid:*` are edit actions.

**Fix options (pick one):**
- Strip `href` from all `xhtml:a` elements (render as plain inline text via CSS).
- Rewrite `d:word` → `/dict?query=word` (same as `entry://word`), strip all others.

The second option gives oald9 clickable cross-references consistent with LDOCE5++.

---

## 3. Custom structural elements

oald9 uses a large set of custom block/inline tags that have no CSS in the current
setup: `<h-g>`, `<top-g>`, `<sn-g>`, `<sn-blk>`, `<def>`, `<x-g-blk>`, `<x>`,
`<chn>`, `<phon>`, `<pron-g-blk>`, `<idm-g>`, `<idm>`, `<ill-g>`, `<ill>`, etc.

The dict ships `oalecd9.css` (already present in `~/.config/ondict/dicts/`) which
styles these elements for the original app. That CSS references fonts and resources
from the MDD. Whether it renders correctly with the current CSS injection approach
(`loadAllCss`) needs verification — `oalecd9.css` is already picked up by
`filepath.WalkDir`.

**Action:** Load the server, query a word from oald9, inspect rendering. May just
work once the audio issues above are fixed.
