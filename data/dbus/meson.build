dbus = dependency('dbus-1')

configure_file(
  configuration: config_data,
	input: 'com.redhat.yggdrasil.Dispatcher1.service.in',
	output: 'com.redhat.yggdrasil.Dispatcher1.service',
  install: true,
  install_dir: dbus.get_pkgconfig_variable('system_bus_services_dir'),
)

install_data('yggd.conf',
  install_dir: join_paths(dbus.get_pkgconfig_variable('datadir'), 'dbus-1', 'system.d')
)

install_data('yggd.conf',
  install_dir: join_paths(dbus.get_pkgconfig_variable('datadir'), 'dbus-1', 'session.d')
)
