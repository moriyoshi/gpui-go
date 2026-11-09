" Run from any directory: vim -Nu NONE -n -i NONE -es -S /path/to/test/syntax.vim
set nocompatible
set shortmess+=F
let s:plugin = fnamemodify(expand('<sfile>:p'), ':h:h')
execute 'set runtimepath^=' . fnameescape(s:plugin)
filetype on
syntax on
execute 'edit ' . fnameescape(s:plugin . '/test/fixture.gox')
call assert_equal('gox', &l:filetype)
call assert_equal('gox', b:current_syntax)

" Assert the syntax Vim actually assigns to a byte, not the regex definitions.
function! s:Check(line, token, expected) abort
  let lines = filter(range(1, line('$')), {_, n -> getline(n) ==# a:line})
  call assert_equal(1, len(lines), 'unique fixture line: ' . a:line)
  let col = stridx(a:line, a:token) + 1
  call assert_true(col > 0, 'fixture token: ' . a:token)
  if len(lines) == 1 && col > 0
    call assert_equal(a:expected, synIDattr(synID(lines[0], col, 1), 'name'), a:line . ' at ' . a:token)
  endif
endfunction

call s:Check('package highlight', 'package', 'goPackage')
call s:Check('// Ordinary Go stays Go.', 'Ordinary', 'goComment')
call s:Check('func ordinary(n int) bool { return n < 10 }', 'return', 'goStatement')
call s:Check('var comparison = 1 < 2', '2', 'goDecimalInt')
call s:Check('var quoted = "<div> ~(false) ~~"', '<div>', 'goString')
call s:Check('var raw = `<span>text</span>`', '<span>', 'goRawString')
call s:Check('elem (u User) Card(show bool, users []User) {', 'elem', 'goxDeclaration')
call s:Check('elem (u User) Card(show bool, users []User) {', 'bool', 'goType')
call s:Check('    count := len(users)', 'len', 'goBuiltins')
call s:Check('    _ = "~~ ) } > </div>"', '~~', 'goString')
call s:Check('    // ~~ is inside a Go comment.', '~~', 'goComment')
call s:Check('    if for return 123 "literal text"', 'if', 'goxBody')
call s:Check('    <div data-id="card > still a string" tabindex=0 (gox.ModifyFunc(nil))>', '<div', 'goxTagDelimiter')
call s:Check('    <div data-id="card > still a string" tabindex=0 (gox.ModifyFunc(nil))>', 'data-id', 'goxAttribute')
call s:Check('    <div data-id="card > still a string" tabindex=0 (gox.ModifyFunc(nil))>', 'still', 'goxAttributeValue')
call s:Check('    <div data-id="card > still a string" tabindex=0 (gox.ModifyFunc(nil))>', 'nil', 'goPredefinedIdentifiers')
call s:Check('        <div><div>nested</div>outer text</div>', 'outer', 'goxElement')
call s:Check('            // ) } > and </custom-tag> stay in this comment.', '</custom-tag>', 'goComment')
call s:Check('            return ") } > </custom-tag>"', 'return', 'goStatement')
call s:Check('            return ") } > </custom-tag>"', '</custom-tag>', 'goString')
call s:Check('        <input value=(u.Name) disabled=(count > 2)/>', '2', 'goDecimalInt')
call s:Check('        <br>after void', 'after', 'goxElement')
call s:Check('        <input title=`raw > attribute` data-code=0xFF data-user=User{Name: "Ada"}>', 'raw', 'goRawString')
call s:Check('        <input title=`raw > attribute` data-code=0xFF data-user=User{Name: "Ada"}>', '0xFF', 'goHexadecimalInt')
call s:Check('        <input title=`raw > attribute` data-code=0xFF data-user=User{Name: "Ada"}>', 'Ada', 'goString')
call s:Check('        ~(len([]string{")", "}"})) after expression', 'after', 'goxElement')
call s:Check('        ~(if show {', 'if', 'goxControl')
call s:Check('            if for return 456', 'if', 'goxBranch')
call s:Check('        } else if count > 0 {', 'else', 'goxElse')
call s:Check('        } else if count > 0 {', '0', 'goDecimalInt')
call s:Check('            no users', 'no', 'goxBranch')
call s:Check('        ~(for _, user := range users {', 'for', 'goxControl')
call s:Check('        ~(for _, user := range users {', 'range', 'goRepeat')
call s:Check('        ~({ if !show { return nil }; return <span>computed</span> })', 'return', 'goStatement')
call s:Check('        ~>(gox.ProxyFunc(func(c gox.Cursor, e gox.Elem) error { return e(c) })) <span>proxied</span>', '~>(', 'goxDelimiter')
call s:Check('        ~"literal ) }" ~42 ~User{Name: "Ada"} ~`raw ) }`', 'literal', 'goString')
call s:Check('        ~"literal ) }" ~42 ~User{Name: "Ada"} ~`raw ) }`', '42', 'goDecimalInt')
call s:Check('        ~"literal ) }" ~42 ~User{Name: "Ada"} ~`raw ) }`', 'Ada', 'goString')
call s:Check('        ~"literal ) }" ~42 ~User{Name: "Ada"} ~`raw ) }`', 'raw', 'goRawString')
call s:Check('        ~// TODO template comment with <div> ~(bad)', 'TODO', 'goxTodo')
call s:Check('            <div> ~(bad)', '<div>', 'goxBlockComment')
call s:Check('        <!-- emitted comment with ~(bad) -->', '~(bad)', 'goxHTMLComment')
call s:Check('        <:><svg>~(raw) ~~ if { }</svg></:>', '~(raw)', 'goxRaw')
call s:Check('        <>fragment <span>child</span></>', 'fragment', 'goxFragment')
call s:Check('    text after root', 'text', 'goxBody')
call s:Check('var inline = <div><div>inline nested</div>inline outer</div>', 'outer', 'goxElement')
call s:Check('var fragment = <>hello ~(42)</>', '42', 'goDecimalInt')
call s:Check('var anonymous = elem() { <span>anonymous</span> }', 'elem', 'goxDeclaration')
call s:Check('    literal signature body', 'literal', 'goxBody')
call s:Check('func after() int { return 123 }', 'return', 'goStatement')

" Loading twice preserves syntax, and generated files retain the Go filetype.
runtime syntax/gox.vim
call assert_equal('gox', b:current_syntax)
new gox-vim-test.x.go
call assert_equal('go', &l:filetype)
new gox-vim-new.gox
call assert_equal('gox', &l:filetype)

if !empty(v:errors)
  for error in v:errors
    echomsg error
  endfor
  cquit
endif
echomsg 'GoX Vim syntax checks passed'
qa!
