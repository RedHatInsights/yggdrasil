project('yggdrasil',
        version: '0.3.0')

go = find_program('go')

if get_option('vendor')
  meson.add_dist_script(join_paths('build-aux', 'vendor.sh'))
endif

goldflags = ''
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.Version=' + meson.project_version() + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DefaultPathPrefix=' + meson.project_name() + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.PrefixDir=' + get_option('prefix') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.SysconfDir=' + get_option('sysconfdir') + '"'
goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.LocalstateDir=' + get_option('localstatedir') + '"'

if get_option('default_data_host') != ''
  goldflags += ' -X "github.com/redhatinsights/yggdrasil/internal/constants.DefaultDataHost=' + get_option('default_data_host') + '"'
endif

gobuildflags = [
  '-buildmode=pie',
]

config_data = configuration_data({
  'sbindir': get_option('sbindir'),
  'configdir': join_paths(get_option('sysconfdir'), meson.project_name())
})

subdir('cmd/yggctl')
subdir('cmd/yggd')
subdir('data')
subdir('doc')

summary(
  {
    'default_data_host': get_option('default_data_host'),
    'vendor': get_option('vendor'),
  },
  section: 'Options'
)
