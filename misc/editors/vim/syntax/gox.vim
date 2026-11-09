" Vim syntax file
" Language: GoX (doors-dev/gox v0.3.3)

if exists('b:current_syntax')
  finish
endif
let s:save_cpo = &cpo
set cpo&vim

" Include the user's Go syntax rather than maintaining another Go lexer.
syntax include @goxGo syntax/go.vim
unlet! b:current_syntax
syntax case match
syntax cluster goxGo remove=goParen,goBlock

syntax cluster goxCode contains=@goxGo,goxParen,goxBrace,goxBracket,goxElement,goxVoidTag,goxFragment,goxRaw,goxDeclaration
syntax cluster goxTemplate contains=goxElement,goxVoidTag,goxFragment,goxRaw,goxExpression,goxSnippet,goxLineComment,goxBlockComment,goxHTMLComment,goxLiteral

" Explicit recursive Go delimiters shield expression ends, including inside
" callbacks, composite literals and nested calls. Strings/comments shield theirs.
syntax region goxFile transparent start=/\%^/ end=/\%$/ contains=@goxCode
syntax region goxParen transparent contained start=/(/ end=/)/ contains=@goxCode
syntax region goxBrace transparent contained start=/{/ end=/}/ contains=@goxCode
syntax region goxBracket transparent contained start=/\[/ end=/\]/ contains=@goxCode

" elem functions and methods switch from Go signatures to template bodies.
syntax keyword goxDeclaration elem contained nextgroup=goxSignature skipwhite skipnl
syntax region goxSignature transparent contained start=/\%(\h\w*\_s*\)\?(/ end=/\ze{/ contains=@goxGo,goxParen,goxBracket nextgroup=goxBody skipwhite skipnl
syntax region goxBody matchgroup=goxDelimiter contained start=/{/ end=/}/ contains=@goxTemplate

" Without a matchgroup, the opening head is also parsed by goxTag. Leaving
" '/>' unconsumed in goxTag lets the enclosing element end at a self close.
syntax region goxTag contained matchgroup=goxTagDelimiter start=+<[[:alpha:]][[:alnum:]_:-]*+ end=+>+ end=+\ze/>+ contains=goxAttribute,goxAttributeValue,goxAttributeEqual,goxModifier
syntax match goxAttribute contained /[[:alpha:]_:@][[:alnum:]_.:@-]*/
syntax match goxAttributeEqual contained /=/ nextgroup=@goxGo,goxAttributeValue,goxModifier,goxLiteralComposite skipwhite skipnl
syntax region goxAttributeValue contained start=/"/ skip=/\\\\\|\\"/ end=/"/
syntax region goxAttributeValue contained start=/'/ skip=/\\\\\|\\'/ end=/'/
syntax region goxModifier matchgroup=goxDelimiter contained start=/(/ end=/)/ contains=@goxCode
syntax region goxElement contained start=+<\z([[:alpha:]][[:alnum:]_:-]*\)\%(\_s\|/\?>\)\@=+ matchgroup=goxTagDelimiter end=+</\z1\s*>+ end=+/>+ contains=goxTag,@goxTemplate

" HTML void elements have no matching end tag (with or without a slash).
syntax region goxVoidTag contained matchgroup=goxTagDelimiter start=+<\%(area\|base\|br\|col\|embed\|hr\|img\|input\|link\|meta\|param\|source\|track\|wbr\)\>+ end=+/\?>+ contains=goxAttribute,goxAttributeValue,goxAttributeEqual,goxModifier
syntax region goxFragment matchgroup=goxTagDelimiter contained start=+<>+ end=+</>+ contains=@goxTemplate
syntax region goxRaw matchgroup=goxDelimiter contained start=+<:>+ end=+</:>+

syntax region goxExpression matchgroup=goxDelimiter contained start=/\~>\?(/ end=/)/ contains=@goxCode,goxControl,goxElse
syntax keyword goxControl if for contained nextgroup=goxCondition,goxBranch skipwhite skipnl
syntax keyword goxElse else contained nextgroup=goxControl,goxBranch skipwhite skipnl
syntax region goxCondition transparent contained start=/[^[:space:]{]/ end=/\ze{/ contains=@goxGo,goxParen,goxBracket nextgroup=goxBranch skipwhite skipnl
syntax region goxBranch matchgroup=goxDelimiter contained start=/{/ end=/}/ contains=@goxTemplate nextgroup=goxElse skipwhite skipnl
syntax region goxSnippet matchgroup=goxDelimiter contained start=/\~\~/ end=/\~\~/ contains=@goxCode

syntax keyword goxTodo TODO FIXME XXX BUG contained
syntax region goxLineComment contained start=+\~//+ end=+$+ contains=goxTodo,@Spell
syntax region goxBlockComment contained start=+\~/\*+ end=+\*/+ contains=goxTodo,@Spell
syntax region goxHTMLComment contained start=+<!--+ end=+-->+ contains=goxTodo,@Spell
syntax match goxLiteral contained /\~\ze\%("\|'\|`\|\d\|[[:alpha:]_]\)/ nextgroup=@goxGo,goxLiteralComposite
syntax region goxLiteralComposite matchgroup=goxDelimiter contained start=/\%(\h\w*\.\)*\h\w*{/ end=/}/ contains=@goxCode

highlight default link goxDeclaration Keyword
highlight default link goxDelimiter PreProc
highlight default link goxTagDelimiter Statement
highlight default link goxElement Normal
highlight default link goxAttribute Type
highlight default link goxAttributeEqual Operator
highlight default link goxAttributeValue String
highlight default link goxControl Conditional
highlight default link goxElse Conditional
highlight default link goxLiteral PreProc
highlight default link goxLineComment Comment
highlight default link goxBlockComment Comment
highlight default link goxHTMLComment Comment
highlight default link goxTodo Todo

syntax sync fromstart
let b:current_syntax = 'gox'
let &cpo = s:save_cpo
unlet s:save_cpo
