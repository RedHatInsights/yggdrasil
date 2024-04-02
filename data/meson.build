subdir('completion')
subdir('dbus')
subdir('systemd')
subdir('yggdrasil')

pkg = import('pkgconfig')
pkg.generate(
  name: meson.project_name(),
  description: 'Remote data transmission and processing client',
  variables: [
    'user='+get_option('user'),
    'worker_user='+get_option('worker_user'),
  ],
)
