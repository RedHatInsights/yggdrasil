project('yggdrasil',
        version: '0.3.2',
        meson_version: '>= 0.58.0')

go = find_program('go')

dbus = dependency('dbus-1', version: '>=1.14')
systemd = dependency('systemd')
bash_completion = dependency('bash-completion')

if get_option('vendor')
  meson.add_dist_script(join_paths('build-aux', 'vendor.sh'))
endif

goldflags = get_option('goldflags')
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.Version=' + meson.project_version() + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DefaultPathPrefix=' + get_option('default_path_prefix') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.PrefixDir=' + get_option('prefix') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.SysconfDir=' + get_option('sysconfdir') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.LocalstateDir=' + get_option('localstatedir') + '"'

if get_option('default_data_host') != ''
  goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DefaultDataHost=' + get_option('default_data_host') + '"'
endif

gobuildflags = get_option('gobuildflags')

config_data = configuration_data({
  'bindir': join_paths(get_option('prefix'), get_option('bindir')),
  'sbindir': join_paths(get_option('prefix'), get_option('sbindir')),
  'configdir': join_paths(get_option('sysconfdir'), meson.project_name()),
  'libexecdir': join_paths(get_option('prefix'), get_option('libexecdir'))
})

subdir('cmd/yggctl')
subdir('cmd/yggd')
subdir('data')
subdir('dbus')
subdir('doc')
subdir('ipc')

if get_option('examples')
  subdir('worker/echo')
endif

summary(
  {
    'default_data_host': get_option('default_data_host'),
    'default_path_prefix': get_option('default_path_prefix'),
    'vendor': get_option('vendor'),
    'examples': get_option('examples')
  },
  section: 'Options'
)
