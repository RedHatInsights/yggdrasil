subdir('system')
subdir('user')

systemd_sysusers_dir = systemd.get_variable(pkgconfig: 'sysusersdir')

configure_file(
  configuration: config_data,
  input: 'yggdrasil-sysuser.conf.in',
  output: 'yggdrasil.conf',
  install: true,
  install_dir: systemd_sysusers_dir,
)
