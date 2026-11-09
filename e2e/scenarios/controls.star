# Native hit testing and key activation must reach Go through Sable, then update
# a Rust-owned model and its observer before the rendered label changes.
app = open_app()
assert_eq(state(app)["count"], 0)
assert_eq(inspect(app, "count")["text"], "Count: 0")
click(app, "increment")
assert_eq(state(app)["count"], 1)
assert_eq(inspect(app, "count")["text"], "Count: 1")
key(app, "enter")
assert_eq(state(app)["count"], 2, "Enter activates the focused native button")

# A click outside the button must not invoke its handler.
click(app, "root", x = 700, y = 760)
assert_eq(state(app)["count"], 2, "missed hitboxes must not become callbacks")

# A pending chord must survive an unrelated Go snapshot replacement.
click(app, "name")
key(app, "cmd-k")
assert_eq(state(app)["count"], 2)
update(app, "rerender")
key(app, "cmd-s")
assert_eq(state(app)["count"], 3)
key(app, "cmd-j")
assert_eq(inspect(app, "count")["text"], "Count: 4")
close_app(app)
