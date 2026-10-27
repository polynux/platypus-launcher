# Fixture font (issue #105, D8)

fixture.woff2 is a tiny subset (basic latin, U+0020-007E) of Hanken Grotesk 400, made with fonttools from the @fontsource/hanken-grotesk file the UI already bundles (apps/ui imports it in style.css). The family stays "Fixture Sans" in theme.toml: a shipped family must not collide with a bundled one, and FontFace registers whatever name the theme declares. Licence: OFL.txt (SIL Open Font License 1.1, Hanken Grotesk Project Authors).
