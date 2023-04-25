install_data('tags.toml',
  install_dir: join_paths(get_option('prefix'), get_option('datadir'), 'doc', meson.project_name())
)

custom_target('yggd.1',
  output: 'yggd.1',
  capture: true,
  command: [yggd, '--generate-man-page'],
  install: true,
  install_dir: join_paths(get_option('mandir'), 'man1')
)

custom_target('yggctl.1',
  output: 'yggctl.1',
  capture: true,
  command: [yggctl, '--generate-man-page'],
  install: true,
  install_dir: join_paths(get_option('mandir'), 'man1')
)
