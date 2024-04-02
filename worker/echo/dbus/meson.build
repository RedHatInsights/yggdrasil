configure_file(
  configuration: config_data,
  input: 'com.redhat.Yggdrasil1.Worker1.echo.service.in',
  output: '@BASENAME@',
  install: true,
  install_dir: dbus.get_variable(pkgconfig: 'system_bus_services_dir')
)

configure_file(
  configuration: config_data,
  input: 'com.redhat.Yggdrasil1.Worker1.echo.conf.in',
  output: '@BASENAME@',
  install: true,
  install_dir: join_paths(dbus.get_variable(pkgconfig: 'datadir'), 'dbus-1', 'system.d')
)
