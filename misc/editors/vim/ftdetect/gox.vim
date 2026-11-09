" GoX source files; leave generated .x.go files to the Go filetype.
augroup gox_filetype
  autocmd!
  autocmd BufRead,BufNewFile *.gox setfiletype gox
augroup END
