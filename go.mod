module github.com/floss-fund/portal

go 1.24.0

toolchain go1.24.1

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/altcha-org/altcha-lib-go v1.0.0
	github.com/floss-fund/go-funding-json v0.6.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/knadh/goyesql/v2 v2.2.0
	github.com/knadh/koanf/parsers/toml v0.1.0
	github.com/knadh/koanf/providers/file v1.2.1
	github.com/knadh/koanf/providers/posflag v1.0.1
	github.com/knadh/koanf/v2 v2.3.2
	github.com/knadh/paginator/v2 v2.0.1
	github.com/knadh/stuffbin v1.3.0
	github.com/labstack/echo/v4 v4.15.0
	github.com/lib/pq v1.11.1
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.11.1
	github.com/zerodha/easyjson v1.0.1
	golang.org/x/mod v0.32.0
	gopkg.in/volatiletech/null.v6 v6.0.0-20170828023728-0bef4e07ae1b
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/imdario/mergo v1.0.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/imdario/mergo => github.com/imdario/mergo v0.3.8
