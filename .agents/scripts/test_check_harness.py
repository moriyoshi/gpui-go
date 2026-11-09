"""Behavioral controls for the harness checker; fixtures never touch source."""

from pathlib import Path
import tempfile
import unittest

from check_harness import check_document, check_skill


class HarnessChecks(unittest.TestCase):
    def setUp(self):
        scratch = Path(__file__).resolve().parents[2] / ".agents-workspace/tmp"
        scratch.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def test_existing_link_then_missing_target(self):
        doc = self.root / "guide.md"
        target = self.root / "api.go"
        target.write_text("package api\n")
        doc.write_text("[API](api.go#ignored-fragment)\n")
        self.assertEqual(check_document(doc, self.root), [])
        target.unlink()
        self.assertTrue(any("missing link target" in error for error in check_document(doc, self.root)))

    def test_external_links_and_unicode_code_are_literal(self):
        doc = self.root / "guide.md"
        doc.write_text("[External](https://example.invalid/no-network)\n[Current page]()\n```text\n[Literal](missing.md)\n\uff08\uff1a\n```\n`\uff08\uff1a`\n")
        self.assertEqual(check_document(doc, self.root), [])
        doc.write_text(doc.read_text() + "Prose\uff1a failure\n")
        self.assertTrue(any("punctuation" in error for error in check_document(doc, self.root)))

    def test_skill_metadata_must_match_directory(self):
        skill = self.root / "test-skill/SKILL.md"
        skill.parent.mkdir()
        skill.write_text("---\nname: test-skill\ndescription: Exercise fixture validation.\n---\n\nInstructions.\n")
        self.assertEqual(check_skill(skill), [])
        skill.write_text(skill.read_text().replace("name: test-skill", "name: wrong-name"))
        self.assertTrue(any("skill name" in error for error in check_skill(skill)))

    def test_unclosed_metadata_is_rejected(self):
        skill = self.root / "SKILL.md"
        skill.write_text("---\nname: incomplete\n")
        self.assertTrue(any("frontmatter" in error for error in check_skill(skill)))


if __name__ == "__main__":
    unittest.main()
