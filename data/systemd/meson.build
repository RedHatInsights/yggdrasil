systemd_system_unit_dir = systemd.get_pkgconfig_variable('systemd_system_unit_dir')
systemd_user_unit_dir = systemd.get_pkgconfig_variable('systemd_user_unit_dir')

configure_file(
  configuration: config_data,
  input: 'yggdrasil.service.in',
  output: 'yggdrasil.service',
  install: true,
  install_dir: systemd_system_unit_dir
)

install_symlink('yggdrasil.service',
  install_dir: systemd_user_unit_dir,
  pointing_to: join_paths(systemd_system_unit_dir, 'yggdrasil.service')
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
