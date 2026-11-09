page = open_page()
# A strict action must reject all three textboxes without mutating any field.
page.get_by_role("textbox").fill("ambiguous")
