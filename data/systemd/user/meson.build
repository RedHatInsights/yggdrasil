systemd_user_unit_dir = systemd.get_variable(pkgconfig: 'systemduserunitdir')

configure_file(
  configuration: config_data,
  input: 'yggdrasil.service.in',
  output: '@BASENAME@',
  install: true,
  install_dir: systemd_user_unit_dir
)
