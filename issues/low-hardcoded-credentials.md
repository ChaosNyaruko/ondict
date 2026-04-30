# auth.go: Hardcoded plaintext credentials and weak session secret

**Severity:** LOW
**File:** `auth.go:10-13`, `server.go:72`

**Description:**
The username/password are hardcoded as plain strings (`user`/`password`) and the
session cookie store key is the literal string `"secret-key"`. Anyone who reads the
source can authenticate to the `/auth` endpoint. The `/auth` endpoint exposes query
history, which is personal data.

**Advice:**
- Read credentials from an environment variable or a config file (outside the repo).
- Generate the cookie store key from a random secret and store it in the config directory, not hardcoded.
- At minimum, document clearly that these must be changed before any network-accessible deployment.
