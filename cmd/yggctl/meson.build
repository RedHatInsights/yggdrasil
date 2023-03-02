yggctl = custom_target('yggctl',
  build_always_stale: true,
  output: 'yggd',
  command: [go, 'build', gobuildflags, '-o', '@OUTPUT@', '-ldflags', goldflags, 'github.com/redhatinsights/yggdrasil/cmd/yggctl'],
  install: true,
  install_dir: get_option('bindir')
)
