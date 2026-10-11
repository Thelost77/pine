# Auth token storage

Spruce stores the Jellyfin access token as plaintext in `config.toml`.
The config file is written with mode `0600`, so local file permissions are the
security boundary.

## Decision

Do not use platform keychains for Spruce token storage.

Do not obfuscate tokens before writing them to config.

Do not delete the stored token just because one saved-login request returns
`401 Unauthorized`.

## Why

Platform keychains added high implementation and support cost for little gain in
a terminal app. They depend on desktop/session services, can fail in headless or
SSH contexts, and made login persistence depend on environment state outside the
app config.

Token obfuscation was also high work for low reward. It was not real encryption
against a local attacker, but it added fragile machine-bound decode state,
migration paths, and failure modes that could force relogin.

Plaintext token storage is easier to inspect, migrate, back up, and debug. The
token is already a bearer secret; users who need stronger protection should
protect the config directory and account access at the OS level.

## Behavior

On successful login, Spruce writes the returned token directly to config.

On startup, Spruce uses the saved token directly when server address, token, and
user id are present.

On authentication failure, Spruce may return to the login screen, but it should
not erase the saved token automatically. A successful login can overwrite stale
credentials.

## Jellyfin device identity

`server.device_name` and `server.device_id` identify this Spruce installation to
Jellyfin. Missing fields are migrated when config loads: the name is based on
the hostname and the ID is a stable hostname-derived value with a random suffix.
Keep `device_id` stable; changing `device_name` does not create a new device.
