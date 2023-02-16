configure_file(
  configuration: config_data,
  input: 'com.redhat.Yggdrasil1.service.in',
  output: 'com.redhat.Yggdrasil1.service',
  install: true,
  install_dir: dbus.get_pkgconfig_variable('system_bus_services_dir')
)

install_data('yggd.conf',
  install_dir: join_paths(dbus.get_pkgconfig_variable('datadir'), 'dbus-1', 'system.d')
)
