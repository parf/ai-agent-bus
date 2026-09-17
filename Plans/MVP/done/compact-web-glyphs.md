# Compact WEB glyph labels

📌 **TL;DR:** 0.5.60 puts the type glyph directly before identity and group
names in WEB instead of repeating a type label on a second line.

## Result

The directory renders `👤 chief@srv1`, and uses the same compact prefix for
daemon-stated Agent and Service kinds. A row without a caller-visible kind stays
unmarked. The Groups page renders `👥 @group` for every group. Each glyph span
has an accessible image label carrying the corresponding type word.

JSON, URLs, filter values, form inputs and ACL expressions remain plain. Other
WEB views and human CLI retain their full `glyph + word` labels.

## Checks

Package tests require each daemon-stated directory kind to place its glyph
immediately before its linked name, reject restoration of the repeated `User`
line, keep credential-only rows unmarked and require every rendered group name
to have the Group glyph. Each visible glyph also carries its type word as an
accessible image label.

The first targeted mutation run receives no credit: a credential-only mutation
invented `🔑`, while the test prohibited only the three older entity glyphs, so
it survived. The corrected assertion requires the bare identity link to be the
first item in an unclassified cell. The corrected run caught **4/4** named
changes: dropping a directory glyph, restoring the repeated type line,
inventing a credential-only prefix and dropping the Group glyph.

Final frozen `src/compact-web-glyphs-smoke.local.sh --slow`: **609 passed, 0
failed**, exit 0; vet and race passed (`tmp/compact-web-glyphs/slow-final.log`,
lines 10–11 and 965). All 198 source/version manifest entries matched after the
run. The frozen and tracked scripts both have SHA-256
`aff1dd8ff54820c09129106b5f0b92c264780cf3394c054629cd78421a82fab3`.

The final build reported 0.5.60 with stamp `parf@parf.us 2026-09-17 15:38:26`.
