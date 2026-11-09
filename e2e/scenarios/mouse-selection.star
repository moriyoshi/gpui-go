app = open_app()
click(app, "name")
type_text(app, "Drag 日本語🙂")
bounds = inspect(app, "name")["text_bounds"]
y = bounds["y"] + bounds["height"] / 2
start = bounds["x"]
end = bounds["x"] + bounds["width"] - 1
mouse_down(app, "name", x = start, y = y)
mouse_move(app, "name", x = end, y = y, pressed = True)
mouse_up(app, "name", x = end, y = y)
assert_eq(inspect(app, "name")["selection"], [0, 10], "drag selects through the emoji in UTF-16 units")
key(app, "backspace")
assert_eq(state(app)["name"], "")
assert_eq(inspect(app, "name")["selection"], [0, 0])
close_app(app)
