yggdrasil_fc = configure_file(
  configuration: config_data,
  input: 'yggdrasil.fc.in',
  output: '@BASENAME@',
)

custom_target(
  'yggdrasil.pp',
  input: ['yggdrasil.te', yggdrasil_fc],
  output: 'yggdrasil.pp',
  build_by_default: true,
  command: [
    files('semodule_package.sh'),
    '@OUTPUT@',
    '@INPUT@',
  ],
  install: true,
  install_dir: get_option('datadir') / 'selinux' / 'packages' / get_option('selinux_policy'),
)

install_data(
  'yggdrasil.if',
  install_dir: get_option('datadir') / 'selinux' / 'devel' / 'include' / 'contrib',
)
