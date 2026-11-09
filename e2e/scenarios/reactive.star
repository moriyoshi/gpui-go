page = open_page("reactive")
counter = page.get_by_test_id("counter")
expect(counter).to_have_text("Count: 0")
assert_eq(page.state()["builds"], 1)
initial_runs = page.state()["count_runs"]
page.get_by_role("button", name = "Increment", exact = True).click()
expect(counter).to_have_text("Count: 1")
assert_eq(page.state()["count_runs"], initial_runs + 1)
details_runs = page.state()["details_runs"]
page.get_by_role("button", name = "Batch +2", exact = True).click()
expect(counter).to_have_text("Count: 3")
assert_eq(page.state()["count_runs"], initial_runs + 2, "two writes in a batch run a binding once")
assert_eq(page.state()["details_runs"], details_runs, "equal memo output suppresses the dependent binding")

name = page.get_by_role("textbox", name = "Name", exact = True)
name.fill("e\u0301日本語🙂")
expect(page.get_by_test_id("greeting")).to_have_text("Hello, e\u0301日本語🙂")
assert_eq(page.state()["count_runs"], initial_runs + 2, "unrelated text input cannot rerun the counter binding")
assert_eq(page.state()["builds"], 1, "GoX template is constructed once")

# A keyed reorder must retain the input's native ID, text, focus and selection.
row = page.get_by_test_id("item-1")
row.fill("Edited")
identity = row.inspect()["element_id"]
selection = row.inspect()["selection"]
position = row.inspect()["bounds"]["y"]
page.update("reverse")
assert_true(row.inspect()["bounds"]["y"] > position, "reorder must move the native row")
assert_eq(row.inspect()["element_id"], identity)
assert_eq(row.inspect()["selection"], selection)
expect(row).to_be_focused()
page.keyboard.insert_text("!")
expect(row).to_have_value("Edited!")

# Hidden branches unsubscribe; reopening creates a fresh owned branch.
page.update("toggle")
expect(page.get_by_test_id("details")).to_be_hidden()
hidden_runs = page.state()["details_runs"]
page.update("increment")
assert_eq(page.state()["details_runs"], hidden_runs)
page.update("toggle")
expect(page.get_by_test_id("details")).to_have_text("Details: Even")
assert_true(page.state()["details_runs"] > hidden_runs)
assert_eq(page.state()["builds"], 1)
page.update("add")
expect(page.get_by_test_id("item-3")).to_have_value("Item 3")
page.update("remove")
expect(page.get_by_test_id("item-2")).to_have_count(0)
expect(row).to_have_value("Edited!")
assert_eq(row.inspect()["element_id"], identity)
page.close()
