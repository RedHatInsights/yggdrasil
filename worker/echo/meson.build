custom_target('echo',
  build_always_stale: true,
  output: 'echo',
  command: [go, 'build', gobuildflags, '-o', '@OUTPUT@', '-ldflags', goldflags, 'github.com/redhatinsights/yggdrasil/worker/echo'],
  install: true,
  install_dir: join_paths(get_option('libexecdir'), meson.project_name())
)

configure_file(
  configuration: config_data,
  input: 'com.redhat.Yggdrasil1.Worker1.echo.service.in',
  output: 'com.redhat.Yggdrasil1.Worker1.echo.service',
  install: true,
  install_dir: dbus.get_variable(pkgconfig: 'system_bus_services_dir')
)
