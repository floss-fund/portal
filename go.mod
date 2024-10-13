module github.com/floss-fund/portal

go 1.22.5

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/floss-fund/go-funding-json v0.0.0-00010101000000-000000000000
	github.com/jmoiron/sqlx v1.3.5
	github.com/knadh/goyesql/v2 v2.2.0
	github.com/knadh/koanf/maps v0.1.1
	github.com/knadh/koanf/parsers/toml v0.1.0
	github.com/knadh/koanf/providers/file v0.1.0
	github.com/knadh/koanf/providers/posflag v0.1.0
	github.com/knadh/koanf/v2 v2.0.1
	github.com/knadh/stuffbin v1.3.0
	github.com/labstack/echo/v4 v4.11.3
	github.com/lib/pq v1.10.0
	github.com/spf13/pflag v1.0.5
	github.com/zerodha/easyjson v1.0.0
	golang.org/x/mod v0.20.0
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/imdario/mergo v0.0.0-00010101000000-000000000000 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/knadh/paginator v1.0.1 // indirect
	github.com/labstack/gommon v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/floss-fund/go-funding-json => /home/kailash/code/go/my/floss.fund/go-funding-json

replace github.com/imdario/mergo => github.com/imdario/mergo v0.3.8
