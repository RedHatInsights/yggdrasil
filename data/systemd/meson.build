systemd_system_unit_dir = systemd.get_variable(pkgconfig: 'systemd_system_unit_dir')
systemd_user_unit_dir = systemd.get_variable(pkgconfig: 'systemd_user_unit_dir')

yggdrasil_service = configure_file(
  configuration: config_data,
  input: 'yggdrasil.service.in',
  output: '@BASENAME@',
)

install_data(yggdrasil_service,
  install_dir: systemd_system_unit_dir
)

install_data(yggdrasil_service,
  install_dir: systemd_user_unit_dir
)

configure_file(
  configuration: config_data,
  input: 'yggdrasil@.service.in',
  output: 'yggdrasil@.service',
  install: true,
  install_dir: systemd_system_unit_dir
)

install_data('yggdrasil-bus@.service',
  install_dir: systemd_system_unit_dir
)

install_data('yggdrasil-bus@.socket',
  install_dir: systemd_system_unit_dir
)
