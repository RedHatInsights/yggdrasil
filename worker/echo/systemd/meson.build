systemd_system_unit_dir = systemd.get_variable(pkgconfig: 'systemdsystemunitdir')

configure_file(
  configuration: config_data,
  input: 'com.redhat.Yggdrasil1.Worker1.echo.service.in',
  output: '@BASENAME@',
  install: true,
  install_dir: systemd_system_unit_dir,
)
