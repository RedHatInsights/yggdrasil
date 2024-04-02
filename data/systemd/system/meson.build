systemd_system_unit_dir = systemd.get_variable(pkgconfig: 'systemdsystemunitdir')

configure_file(
  configuration: config_data,
  input: 'yggdrasil.service.in',
  output: '@BASENAME@',
  install: true,
  install_dir: systemd_system_unit_dir
)

configure_file(
  configuration: config_data,
  input: 'yggdrasil@.service.in',
  output: '@BASENAME@',
  install: true,
  install_dir: systemd_system_unit_dir
)

configure_file(
  configuration: config_data,
  input: 'yggdrasil-bus@.service.in',
  output: '@BASENAME@',
  install: true,
  install_dir: systemd_system_unit_dir
)

install_data('yggdrasil-bus@.socket',
  install_dir: systemd_system_unit_dir
)
