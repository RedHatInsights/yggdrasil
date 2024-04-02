custom_target('echo',
  build_always_stale: true,
  output: 'echo',
  command: [go, 'build', gobuildflags, '-o', '@OUTPUT@', '-ldflags', goldflags, 'github.com/redhatinsights/yggdrasil/worker/echo'],
  install: true,
  install_dir: join_paths(get_option('libexecdir'), meson.project_name())
)

subdir('dbus')
subdir('systemd')
