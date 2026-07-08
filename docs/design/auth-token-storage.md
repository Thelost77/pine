# Auth token storage

Pine stores the Audiobookshelf access token as plaintext in the local SQLite
database. The database lives under the user's config directory, so local file
permissions are the security boundary.

## Decision

Do not use platform keychains for Pine token storage.

Do not obfuscate tokens before writing them to the account database.

Do not delete the stored token just because one request returns
`401 Unauthorized`.

## Why

Platform keychains added high implementation and support cost for little gain in
a terminal app. They depend on desktop/session services, can fail in headless or
SSH contexts, and made login persistence depend on environment state outside the
app database.

Token obfuscation was also high work for low reward. It was not real encryption
against a local attacker, but it added fragile machine-bound decode state,
migration paths, and failure modes that could force relogin.

Plaintext token storage is easier to inspect, migrate, back up, and debug. The
token is already a bearer secret; users who need stronger protection should
protect the config directory and account access at the OS level.

## Behavior

On successful login, Pine writes the returned token directly to the default
account row.

On startup, Pine uses the saved token directly when a default account has one.

On authentication failure, Pine may return to the login screen, but it should
not erase the saved token automatically. A successful login can overwrite stale
credentials.
