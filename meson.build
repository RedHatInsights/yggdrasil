project('yggdrasil', version : '0.4.8', meson_version : '>= 0.58.0')

go = find_program('go')

dbus = dependency('dbus-1', version: '>=1.12')
systemd = dependency('systemd', version: '>=239')
bash_completion = dependency('bash-completion')

if get_option('vendor')
  meson.add_dist_script(join_paths('build-aux', 'vendor.sh'))
endif

goldflags = get_option('goldflags')
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.Version=' + meson.project_version() + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DefaultPathPrefix=' + get_option('default_path_prefix') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DefaultFactsFile=' + get_option('default_facts_file') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.PrefixDir=' + get_option('prefix') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.SysconfDir=' + get_option('sysconfdir') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.LocalstateDir=' + get_option('localstatedir') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DataDir=' + get_option('datadir') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.LibDir=' + get_option('libdir') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DBusSystemServicesDir=' + dbus.get_variable(pkgconfig: 'system_bus_services_dir') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DBusPolicyConfigDir=' + join_paths(dbus.get_variable(pkgconfig: 'datadir'), 'dbus-1', 'system.d') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.SystemdSystemServicesDir=' + systemd.get_variable(pkgconfig: 'systemdsystemunitdir') + '"'


if get_option('default_data_host') != ''
  goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DefaultDataHost=' + get_option('default_data_host') + '"'
endif

gobuildflags = get_option('gobuildflags')

config_data = configuration_data({
  'bindir': join_paths(get_option('prefix'), get_option('bindir')),
  'sbindir': join_paths(get_option('prefix'), get_option('sbindir')),
  'configdir': join_paths(get_option('sysconfdir'), meson.project_name()),
  'libexecdir': join_paths(get_option('prefix'), get_option('libexecdir')),
  'localstatedir': join_paths(get_option('prefix'), get_option('localstatedir')),
  'sharedstatedir': join_paths(get_option('prefix'), get_option('sharedstatedir')),
  'user': get_option('user'),
  'worker_user': get_option('worker_user'),
})

subdir('cmd/yggctl')
subdir('cmd/yggd')
subdir('data')
subdir('dbus')
subdir('doc')
subdir('dist')
subdir('ipc')

if get_option('examples')
  subdir('worker/echo')
endif

summary(
  {
    'default_data_host': get_option('default_data_host'),
    'default_path_prefix': get_option('default_path_prefix'),
    'default_facts_file': get_option('default_facts_file'),
    'vendor': get_option('vendor'),
    'examples': get_option('examples'),
    'user': get_option('user'),
    'worker_user': get_option('worker_user'),
  },
  section: 'Options'
)
