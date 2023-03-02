yggd = custom_target('yggd',
  build_always_stale: true,
  output: 'yggd',
  command: [go, 'build', gobuildflags, '-o', '@OUTPUT@', '-ldflags', goldflags, 'github.com/redhatinsights/yggdrasil/cmd/yggd'],
  install: true,
  install_dir: get_option('bindir')
)
