# Deliberately failing behavioral control. Excluded from the passing suite.
app = open_app()
click(app, "increment")
assert_eq(state(app)["count"], 99, "negative control must fail after a real native click")
