# GoX syntax for Vim

This runtime plugin highlights `.gox` templates for the repository's pinned GoX v0.3.3. It uses Vim's installed Go syntax for ordinary Go and embedded expressions, and adds `elem` declarations, tags and attributes, fragments, `~(...)` expressions, template if/for branches, `~~` snippets, literal placeholders, proxies, template/HTML comments and opaque `<:>` raw blocks. Nested calls, callbacks, composite literals and repeated tag names are handled recursively. Generated `.x.go` files keep the Go filetype.

Install the contents of this directory as a Vim package, for example by copying it to `~/.vim/pack/local/start/gox`. The resulting paths should include `~/.vim/pack/local/start/gox/ftdetect/gox.vim` and `~/.vim/pack/local/start/gox/syntax/gox.vim`. Enable filetype detection and syntax in your vimrc:

```vim
filetype plugin indent on
syntax enable
```

For an existing checkout, you can instead add its absolute path to your vimrc before those commands:

```vim
set runtimepath^=/absolute/path/to/gpui-go/misc/editors/vim
```

Reopen a `.gox` file after installation. `:set filetype?` should report `gox`. Highlight groups use default Vim colors; override groups such as `goxDeclaration`, `goxTagDelimiter`, `goxAttribute` and `goxDelimiter` in your colorscheme. Existing Go highlighting options apply to the included Go syntax.

This is regex syntax highlighting rather than a parser. Template control-flow headers treat the first brace outside parentheses/brackets as the body opener; put a composite literal in parentheses when using one in such a header. Raw blocks remain literal and do not embed HTML, JavaScript or CSS highlighting. Whole-file synchronization preserves multiline state at the cost of extra work in very large files.

Run the display-independent highlighting checks from the repository root:

```sh
make test-vim
```

The tests use Vim's actual syntax groups, including checks that Go-like template text stays literal and that ordinary Go highlighting resumes after templates. They need Vim with syntax support and its standard Go runtime, and do not need GoX or a native GPUI build. You can also run `vim -Nu NONE -n -i NONE -es -V1 -S misc/editors/vim/test/syntax.vim` directly. The fixture is GoX source; generate it only in scratch space if checking it with the compiler.
