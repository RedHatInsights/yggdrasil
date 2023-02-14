systemd = dependency('systemd')
systemd_system_unit_dir = systemd.get_pkgconfig_variable('systemd_system_unit_dir')

configure_file(
  configuration: config_data,
  input: 'yggd.service.in',
  output: 'yggd.service',
  install: true,
  install_dir: systemd_system_unit_dir
)

configure_file(
  configuration: config_data,
  input: 'yggd@.service.in',
  output: 'yggd@.service',
  install: true,
  install_dir: systemd_system_unit_dir
)

install_data('yggdrasil-bus@.service',
  install_dir: systemd_system_unit_dir
)

install_data('yggdrasil-bus@.socket',
  install_dir: systemd_system_unit_dir
)
