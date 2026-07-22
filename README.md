# pelagica-studios

A small Go tool that builds a lookup from company name to logo, covering both movie/TV production companies and TV networks, sourced from TMDB. It is meant to be used as a data source for other apps, for example a Jellyfin client that wants to show a studio's or network's logo next to its name.

## How it works

The tool downloads TMDB's daily id exports for production companies and TV networks, then fetches full details (name, logo, headquarters, etc.) for every id and stores them in a single local SQLite database, in a `companies` table with a `type` column set to either `production_company` or `tv_network`.

On every run it:

- Fetches details for any id that is not yet in the database, for both entity types.
- Refreshes a rolling batch of the least recently fetched existing entries across both types, so that every entry gets re-checked well within TMDB's cache retention limit instead of requiring one long, infrequent full re-scan.
- Removes entries that TMDB no longer returns data for.
- Writes the database back out as a matching JSON export.

A GitHub Actions workflow (`.github/workflows/snapshot.yml`) runs this daily, downloads the previous snapshot as a starting point, runs the tool, and publishes the result as a new dated GitHub release. Older snapshots that predate the `companies` table (back when this only tracked production companies under a `production_companies` table) are migrated into the current schema automatically the first time such a database is opened; nothing manual is needed.

## Releases

Each release contains:

- `companies.db`, the full SQLite database.
- `companies_logosonly.db`, the same data with only entries that have a logo.
- `companies_added.md`, a short report listing entries added since the previous release (broken down by production companies vs tv networks), how many of those have a logo, and how many existing entries were refreshed in that run.

## Running locally

Requires Go and a TMDB API read access token set as `TMDB_API_KEY` (an `.env` file is loaded automatically).

Warning: if `data/companies.db` does not already exist, the tool has no starting point and will treat every production company and tv network TMDB knows about, currently around 250,000 combined, as new. At TMDB's rate limit that run takes on the order of half a day. Before running without a starting point, either download a snapshot from the latest release into `data/` first, or set `COMPANY_LIMIT` to a small number, as `task dev` already does below.

```
task dev    # go run ., limited to a few companies for quick testing
task build  # builds bin/pelagica-studios
task start  # builds if needed, then runs the built binary
```

## License

The code in this repository is licensed under the Apache License 2.0, see [LICENSE](LICENSE). This license covers the code only. The data fetched and republished by this tool is TMDB content and remains subject to TMDB's own terms, described below.

## Disclaimers and attribution

This project is not affiliated with, endorsed by, certified by, or in any way officially connected with TMDB (The Movie Database) or any of the production companies, studios, or other entities whose names, logos, or other data appear in the fetched data.

This product uses the TMDB API but is not endorsed or certified by TMDB.

Data obtained through the TMDB API, including company names and logos, belongs to TMDB and its contributors, not to this project. Anyone using the published database snapshots is responsible for complying with TMDB's own API terms of use, including its attribution and logo requirements, available at https://www.themoviedb.org/about/logos-attribution and https://www.themoviedb.org/api-terms-of-use. Any application that displays data or logos sourced from these snapshots should include its own TMDB attribution notice and logo, as required by those terms.

The data is provided as is, without warranty of any kind, including accuracy, completeness, or fitness for a particular purpose. Company names, logos, and other details reflect what TMDB returned at the time of fetching and may be outdated, incorrect, or removed at any time.
