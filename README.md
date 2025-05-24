# Cloudflare D1 Grafana Datasource

[![Release](https://img.shields.io/github/v/release/olipayne/grafana-cloudflare-d1-datasource?style=flat-square)](https://github.com/olipayne/grafana-cloudflare-d1-datasource/releases/latest)
[![Software License](https://img.shields.io/badge/license-Apache2-brightgreen.svg?style=flat-square)](LICENSE)

<!-- Add other badges as appropriate: build status, etc. -->

This plugin allows Grafana to connect to [Cloudflare D1](https://developers.cloudflare.com/d1/) databases as a data source, enabling you to query and visualize your D1 data within Grafana dashboards.

## Features

- **Connect to Cloudflare D1:** Configure your D1 Account ID, Database ID, and API Token to access your database.
- **Execute SQL Queries:** Write standard SQL queries in the Grafana query editor to fetch data from your D1 tables.
- **Visualize Data:** Leverage Grafana\'s rich visualization capabilities (tables, charts, etc.) with your D1 data.
- **Basic Type Inference:** Attempts to infer data types (numbers, strings, booleans, timestamps in RFC3339Nano format) from query results.

## Requirements

- Grafana 9.x or later (Please update if a different version is targeted by the SDK used).
- A Cloudflare account with a D1 database.
- A Cloudflare API Token with permissions to read the D1 database.

## Setup & Configuration

1.  **Install the Plugin:**

    - **From Release (Recommended for users):** Download the latest release `zip` file from the [Releases page](https://github.com/olipayne/grafana-cloudflare-d1-datasource/releases) (replace with actual link once you have one). Unzip it into your Grafana plugin directory (e.g., `/var/lib/grafana/plugins` on Linux, or `data/plugins` relative to your Grafana install).
    - **Manual Build (For developers):** See the [Development](#development) section.
    - Restart your Grafana server after installing the plugin.

2.  **Gather Cloudflare D1 Credentials:**

    - **Account ID:**
      1.  Log in to your Cloudflare dashboard.
      2.  Navigate to "Workers & Pages".
      3.  Your Account ID is typically found in the URL (e.g., `https://dash.cloudflare.com/{account_id}/...`) or on the right sidebar under "Account ID".
      <!-- TODO: User to verify and add more precise instructions or link to CF docs -->
    - **Database ID:**
      1.  In the Cloudflare dashboard, go to "Workers & Pages" -> "D1".
      2.  Select your desired D1 database.
      3.  The Database ID is usually displayed prominently on the database\'s overview page. It might be referred to as "Database ID" or similar.
      <!-- TODO: User to verify and add more precise instructions or link to CF docs -->
    - **API Token:**
      1.  In the Cloudflare dashboard, go to "My Profile" (top right) -> "API Tokens".
      2.  Click "Create Token".
      3.  You can start with the "Edit Cloudflare Workers" template or create a custom token.
      4.  Ensure the token has at least the following permissions:
          - **Account Resources:** `D1`: `Read` (This grants read access to all D1 databases in the account. You might want to create more scoped tokens if Cloudflare allows it in the future for specific D1 DBs).
          <!-- TODO: User to verify exact minimum permissions. CF docs on D1 API auth would be helpful. -->
      5.  Give your token a descriptive name.
      6.  Click "Continue to summary" and then "Create Token".
      7.  **Important:** Copy the generated API token immediately. You will not be able to see it again.

3.  **Add Data Source in Grafana:**
    1.  In Grafana, go to "Connections" (or "Configuration" in older versions) -> "Data sources".
    2.  Click "Add new data source".
    3.  Search for "Cloudflare D1" (or the name you set in `plugin.json`) and select it.
    4.  Enter the following details:
        - **Name:** A descriptive name for this data source instance (e.g., "My D1 Prod DB").
        - **Account ID:** Your Cloudflare Account ID.
        - **Database ID:** Your Cloudflare D1 Database ID.
        - **API Token:** Your Cloudflare API Token (this is a secret and will be encrypted).
    5.  Click "Save & test". You should see a message like "Successfully connected to Cloudflare D1 and executed test query."

## Usage

1.  **Create a Panel:** Go to a dashboard and add a new panel.
2.  **Select Data Source:** Choose the Cloudflare D1 data source you configured.
3.  **Write SQL Query:** In the query editor, enter your SQL query.
    ```sql
    SELECT
        column1,
        column2,
        timestamp_column
    FROM
        your_table
    WHERE
        some_condition = \'value\'
    ORDER BY
        timestamp_column DESC
    LIMIT 100;
    ```
4.  **Visualize:** Choose a visualization (e.g., Table, Time series) and configure it.

### Querying Notes & Limitations

- **Column Ordering:** Columns in the Grafana table/results are currently sorted alphabetically by column name, not by the order in your `SELECT` statement. This is due to how data is processed from the D1 API.
- **Timestamp Handling:** The plugin attempts to detect timestamp columns if they are strings formatted according to RFC3339Nano (e.g., `2023-10-26T07:30:00.123456789Z`). Other timestamp formats might be treated as plain strings or numbers. For time-series visualizations, ensure your timestamp column is correctly identified.
- **Type Inference:** Data types are inferred from the first row of the result set. If the first row has a `NULL` value for a column, that column may default to a string type.

## Development

1.  **Prerequisites:**

    - [Node.js](https://nodejs.org/) (LTS version recommended)
    - [Go](https://golang.org/doc/install) (version specified in `Dockerfile` or `go.mod`)
    - [Mage](https://magefile.org/) (Go build tool): `go install github.com/magefile/mage@latest`
    - [Docker](https://www.docker.com/) & Docker Compose (for running Grafana locally)

2.  **Clone the Repository:**

    ```bash
    git clone https://github.com/olipayne/grafana-cloudflare-d1-datasource.git
    cd grafana-cloudflare-d1-datasource
    ```

3.  **Install Dependencies:**

    - Frontend: `npm install`
    - Backend: `go mod tidy` (should already be tidy)

4.  **Build Plugin:**

    - Build frontend (watches for changes): `npm run dev`
    - Build backend (replace `darwinARM64` with your OS/architecture, e.g., `linuxAMD64`, `windowsAMD64`):
      ```bash
      mage -v build:darwinARM64
      ```
      Common targets: `build:linuxAMD64`, `build:linuxARM64`, `build:darwinAMD64`, `build:darwinARM64`, `build:windowsAMD64`.
      Run `mage -l` to see all available build targets.
    - To build all backend binaries: `mage -v buildAll`

5.  **Run Grafana with Plugin Locally:**
    - Ensure `npm run dev` is running in one terminal.
    - Ensure your backend binary (e.g., `dist/gpx_cloudflare_d1_datasource_linux_arm64` if your Docker runs Linux/ARM64) is built and matches the architecture expected by the Docker container (see `Dockerfile` `TARGETARCH`).
    - In another terminal, start Grafana using Docker Compose:
      ```bash
      docker compose up
      ```
    - Access Grafana at `http://localhost:3000` (admin/admin).
    - The plugin will be available. The `docker-compose.yaml` volume mounts the `dist` directory, so backend changes require a rebuild and then a Grafana restart (or Docker Compose restart) for Grafana to pick up the new backend binary. Frontend changes from `npm run dev` are usually picked up with a browser refresh.

## Troubleshooting

- **"Plugin not found" in Grafana logs / UI:**
  - Ensure the plugin `dist` directory is correctly placed in your Grafana plugins directory and Grafana was restarted.
  - Verify the plugin ID in `src/plugin.json` matches what Grafana expects.
  - Check Grafana server logs for more detailed errors.
- **"Plugin failed to start" (when clicking on the data source settings or a panel):**
  - This usually indicates an issue with the backend plugin binary.
  - **Architecture Mismatch:** Ensure the Go backend binary in the `dist` folder (e.g., `gpx_cloudflare_d1_datasource_linux_arm64`) matches the architecture of the Grafana server environment (or the Docker container running Grafana). If you built `darwinARM64` but your Docker Grafana is `linux/amd64`, it won\'t work. Rebuild for the correct architecture.
  - Check Grafana server logs for errors like "fork/exec ...: exec format error" (architecture mismatch) or other Go runtime errors.
- **"API key not valid" or Authentication Errors:**
  - Double-check your API Token, Account ID, and Database ID in the data source configuration.
  - Ensure your API Token has the correct permissions for D1 access.
  - Test the API token directly using `curl` against the D1 API if issues persist.
- **Data Source "Save & Test" works, but queries fail or show no data:**
  - Verify your SQL query syntax is correct for D1 (SQLite).
  - Check Grafana panel inspector for any error messages returned by the data source.
  - Enable debug logging in Grafana and check the Grafana server logs for detailed logs from the plugin.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This plugin is licensed under the MIT. See the [LICENSE](./LICENSE) file for details.

<!-- Optional:
## Acknowledgements
* ...
-->
