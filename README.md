# funding.json manifest portal

An open web directory and portal and a directory for [funding.json](https://fundingjson.org) manifests. Single binary Go application that uses Postgres as its data store.

See the official instance in action at **[dir.floss.fund](https://dir.floss.fund)**.

## Installation

### Postgres
If you don't have an existing Postgres instance running, use the [docker-compose.yml](https://github.com/floss-fund/portal/blob/master/docker-compose.yml) file and run `docker compose up db` to run a new instance. The database name, username, and password for the Docker instance is `portal`.

### Binary
- Download the binary from the [releases](https://github.com/floss-fund/portal/releases) page.
- Run `./portal --new-config` to generate a new TOML config file. Edit the config, primarily the Postgres `[db]` credentials.
- Run `./portal --install` to install the database schema.
- Run `./portal` and visit `localhost:9000`

### Running the crawler
Schedule a cron job to run (`./portal --mode=crawl`) the crawler at the desired interval. The crawler runs N workers and goes through all the manifest URLs in the database and updates their contents if they have changed (based on the Last-Updated header) within the interval specified in the config.
