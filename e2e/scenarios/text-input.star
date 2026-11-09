# These strings intentionally contain a decomposed accent and a surrogate pair.
app = open_app()
click(app, "name")
type_text(app, "e\u0301🙂日本語")
assert_eq(state(app)["name"], "e\u0301🙂日本語")
assert_eq(inspect(app, "name")["value"], "e\u0301🙂日本語")
assert_eq(inspect(app, "name")["selection"], [7, 7], "selection uses UTF-16 offsets")
assert_eq(inspect(app, "name")["focused_element_id"], 10)

# Go echoes must preserve the native caret; deletion follows grapheme boundaries.
key(app, "home right delete")
assert_eq(state(app)["name"], "e\u0301日本語", "Delete removes the entire emoji")
key(app, "backspace")
assert_eq(state(app)["name"], "日本語", "Backspace removes the combined accent")
key(app, "end shift-left")
assert_eq(inspect(app, "name")["selection"], [2, 3])
type_text(app, "!")
assert_eq(state(app)["name"], "日本!")
assert_eq(inspect(app, "mirror")["text"], "Name: 日本!")

# An authoritative epoch reset preserves the persistent entity and focus.
before = inspect(app, "name")["epoch"]
update(app, "replace", "Reset")
assert_eq(inspect(app, "name")["epoch"], before + 1)
assert_eq(inspect(app, "name")["focused_element_id"], 10)
type_text(app, " OK")
assert_eq(state(app)["name"], "Reset OK")

# Tab skips the disabled field; input is delivered to the second native field.
key(app, "tab tab")
assert_eq(inspect(app, "other")["focused_element_id"], 14)
type_text(app, "Second")
assert_eq(state(app)["other"], "Second")
assert_eq(inspect(app, "disabled")["value"], "Locked")
key(app, "shift-tab")
# The reset button is between the fields in tab order.
assert_eq(inspect(app, "reset")["focused_element_id"], 12)
close_app(app)
