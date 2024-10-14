# funding.json manifest portal

An open web directory and portal and a directory for [funding.json](https://floss.fund/funding-manifest) manifests. Single binary Go application that uses Postgres as its data store and Typesense for search.

See the official instance in action at **[dir.floss.fund](https://dir.floss.fund)**.

## Installation

### Binary
- Download the binary from the releases page.
- Run `./portal --new-config` to generate a new TOML config file. Edit the file.
- Run `./portal --install` to install the Postgres and Typesense schemas.
- Run `./portal` and visit `localhost:9000`

### Docker
- Copy the `docker-compose.yml` file.
- Run `docker-compose up`
- Visit `localhost:9000`

### Running the crawler
Schedule a cron job to run (`./portal --mode=crawl`) the crawler at the desired interval. The crawler runs N workers and goes through all the manifest URLs in the database and updates their contents if they have changed (based on the Last-Updated header) within the interval specified in the config.
