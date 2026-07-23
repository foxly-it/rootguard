# RootGuard Updater

`rootguard-updater` is the internal lifecycle helper for the RootGuard control
plane. It is deliberately separate from Core and WebApp so it stays available
while those two containers are replaced.

The helper:

- accepts only bearer-authenticated requests from the internal control network;
- manages only the fixed Compose services `core` and `webapp`;
- obtains target images exclusively from environment configuration;
- compares actual Docker image IDs instead of tags;
- replaces and verifies Core and WebApp as a pair;
- pins both previous image IDs if either health check fails;
- persists status in the protected `rootguard-data` volume;
- exposes no host port.

It does not accept image names, container names, Compose files, or command
arguments through its HTTP API. `ROOTGUARD_UPDATER_SKIP_PULL` exists only for
local/integration tests using prebuilt images and must remain disabled for
release deployments.

## License

RootGuard Updater is licensed under the GNU Affero General Public License v3.0
or later (AGPL-3.0-or-later). See the `LICENSE` file for full details.
