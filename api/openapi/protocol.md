# Illarin platform integration guide

This guide is for developers adding Illarin support to an application or a new
asset platform. The machine-readable contract is served at `/openapi.yaml`. If
this guide and that contract differ, follow OpenAPI and report the mismatch.
All API paths below are relative to the exact Illarin base URL and include the
public `/api` prefix.

The current protocol links an application installation, rotates its credentials,
records what it can accept, lets its owner send assets to it, mirrors what it has
installed, and lets its owner revoke it.

## What one installation must keep

Keep one record per Illarin account and installation:

- Illarin base URL. Never assume one global host.
- Linked-instance ID.
- Granted scopes.
- Current access token and its expiry.
- Current refresh token.
- The declaration last sent to Illarin.

Store refresh tokens in the operating system credential store where one exists.
On a headless system, use a file readable only by the service account or an
equivalent secret store. Do not put tokens in logs, URLs, crash reports, update
checks, analytics, or exported application settings.

Every installation links independently. Do not ship a shared credential and do
not copy one when cloning an application profile, container, or virtual machine.
Clear Illarin credentials when an installation identity is cloned.

Treat the exact base URL as part of the security identity. Send a code or token
only to the server that issued it, and never merge records because two servers,
applications, or installations use the same display name.

## Security model

Illarin authenticates the account owner and the installation they approve. It
does not authenticate or endorse the software vendor. There is no client ID,
client secret, registration, allowlist, or trusted application badge.

The application and installation names on the approval screen are self-asserted
and marked unverified. Never use either name as an identity. Illarin does not
accept application-supplied HTML, CSS, JavaScript, logos, update URLs, or remote
images through this protocol.

Use HTTPS for every request except the final same-device loopback callback. Treat
all codes and tokens as opaque. Generate security values with a cryptographic
random-number generator.

This profile does not make a compromised installation safe. Access and refresh
tokens are bearer secrets, so malware that can read the application's credential
store can act as that installation until the token expires or the owner revokes
it. Manual code matching reduces cross-device consent phishing but cannot protect
an owner who approves an unsolicited matching request. Keep those limits visible
in the application instead of describing a link as absolute proof of platform
identity.

## Implementation order

1. Define one durable local installation record.
2. Choose the narrowest scopes and declare supported export targets.
3. Implement browser authorization with S256 PKCE.
4. Add the manual device fallback only if the application can run where loopback
   is impossible.
5. Serialize refreshes so only one refresh is in flight per installation.
6. Add declaration updates after application upgrades.
7. Refresh once after an access-endpoint `401`; treat a refresh-endpoint `401`
   as terminal and offer to link again.
8. Run one delivery wait at a time and acknowledge only what you installed.
9. Report your library incrementally, with an occasional full snapshot.
10. Run the conformance checklist at the end of this guide.

## Describe the installation

Both authorization paths start with the same declaration:

```json
{
  "applicationName": "Paper Lantern",
  "instanceName": "studio workstation",
  "applicationVersion": "4.2.0",
  "protocolVersion": 1,
  "capabilities": ["org.example.paperlantern:media-sidecars"],
  "acceptedTargets": ["example_bundle_v2", "example_bundle_v1"],
  "scopes": ["asset:receive"]
}
```

The limits are part of the wire contract:

- `applicationName` and `instanceName` are printable text from 1 to 64
  characters after trimming.
- `applicationVersion` is optional and at most 64 characters.
- `protocolVersion` is currently exactly `1`.
- `capabilities` and `acceptedTargets` are required arrays, even when empty.
- Each array has at most 32 unique values, each at most 64 characters.
- A capability has a namespace and name, such as
  `org.example.paperlantern:media-sidecars`.
- An export target is a lowercase module ID such as `example_bundle_v2`.
- The whole JSON request body may not exceed 4 KiB. Unknown JSON fields are
  rejected.

Use a stable reverse-domain namespace for capabilities you own. A capability is
only a claim about interoperability. It does not grant a permission, make an
unknown server feature available, or cause Illarin to run application code.

`acceptedTargets` is ordered from most to least preferred. It tells delivery
which Illarin exports your application can read. The `example_*` values in
this guide are placeholders, not registered targets. Declare only IDs backed by
readers your application actually ships. Illarin exposes the formats currently
offered for an asset in that asset's `downloads[].format` values; there is no
global target catalog in this protocol version. The list can grow, and an unknown
ID grants nothing or selects no server-side writer.

The only scopes are:

- `asset:receive`: collect assets the owner sends to this installation.
- `library:sync`: report this installation's local library state.

Ask only for scopes the installation will use. A declaration update cannot add
or change scopes.

## Same-device browser authorization

This is the normal path for a desktop application. The application opens the
system browser and listens temporarily on loopback.

### 1. Prepare the callback and PKCE values

Bind a listener to `127.0.0.1` or `::1` on an available port. Pick one callback
path and keep the URI byte-for-byte for the exchange. Illarin accepts only:

```text
http://127.0.0.1:<port>/<path>
http://[::1]:<port>/<path>
```

It rejects `localhost`, alternate textual forms of the IP addresses, LAN and
public hosts, missing ports, user information, query strings, and fragments.

Generate:

- A PKCE verifier containing 43 to 128 unreserved characters.
- A state value containing 32 to 128 unreserved characters.
- The S256 challenge:

```text
BASE64URL-NO-PADDING(SHA256(ASCII(code_verifier)))
```

Keep the verifier and state in memory or protected temporary storage. Never put
the verifier in the browser URL.

### 2. Start authorization

```http
POST /api/v1/link/authorizations
Content-Type: application/json

{
  "applicationName": "Paper Lantern",
  "instanceName": "studio workstation",
  "applicationVersion": "4.2.0",
  "protocolVersion": 1,
  "capabilities": [],
  "acceptedTargets": ["example_bundle_v2"],
  "scopes": ["asset:receive"],
  "redirectUri": "http://127.0.0.1:49152/illarin/callback",
  "state": "<random-state>",
  "codeChallenge": "<S256-challenge>",
  "codeChallengeMethod": "S256"
}
```

The response contains `authorizationUrl` and `expiresAt`. The request expires in
five minutes. Open `authorizationUrl` in the system browser. Do not fetch it in
an embedded web view and do not log it; the URL contains a one-use request secret.

### 3. Validate the loopback callback

After approval, the browser requests the exact callback with `code` and `state`
query parameters. After denial it sends `error=access_denied` and `state`.

Before accepting a code:

1. Require the callback path you opened.
2. Compare state with the original value using a constant-time comparison.
3. Reject missing, repeated, or unexpected parameters.
4. Stop the loopback listener after one terminal callback or timeout.

### 4. Exchange the code

```http
POST /api/v1/link/token
Content-Type: application/json

{
  "authorizationCode": "<code-from-the-callback>",
  "codeVerifier": "<original-verifier>",
  "redirectUri": "http://127.0.0.1:49152/illarin/callback"
}
```

The authorization code is one-use. The verifier, exact redirect URI, expiry,
approval, and redemption state must all match. On any failure, discard the local
authorization state and start again. Never retry a successful code exchange.

## Headless device fallback

Use this path only when the installation cannot receive loopback, such as a
remote terminal or headless server. A desktop application must use PKCE instead.

This registration-free flow adopts RFC 8628's manual-code, expiry, polling,
`slow_down`, denial, and consent-phishing protections. It is not a drop-in OAuth
Device Authorization Grant: it has no client ID or `grant_type`, and its HTTP
statuses and response bodies are defined by Illarin's OpenAPI contract.

### 1. Start the device request

```http
POST /api/v1/link/requests
Content-Type: application/json

{
  "applicationName": "Paper Lantern",
  "instanceName": "render box",
  "protocolVersion": 1,
  "capabilities": [],
  "acceptedTargets": ["example_bundle_v2"],
  "scopes": ["asset:receive"]
}
```

The response contains a private `deviceCode`, a short `userCode`,
`verificationUrl`, `expiresAt`, and `interval`. The request expires in ten
minutes.

Show the URL and code separately. There is deliberately no complete prefilled
verification URL. Tell the owner to type the code, confirm that the approval
screen shows the same code, and decline any request they did not start.

### 2. Poll with finite requests

```http
POST /api/v1/link/poll
Content-Type: application/json

{"deviceCode":"<private-device-code>"}
```

Use this state machine:

| Response | Meaning | Next action |
| --- | --- | --- |
| `200`, `status: pending` | No decision yet | Wait at least the current interval |
| `200`, `status: linked` | Link complete | Persist the returned token pair |
| `400`, `access_denied` | Owner declined | Stop |
| `400`, `expired_token` | Request expired | Stop and offer to restart |
| `404` | Unknown or already used code | Stop |
| `429`, `slow_down` | Polling was too fast | Use `Retry-After`; the larger interval remains in force |
| Other `429` | Source rate limit | Use `Retry-After` |
| Network failure | Outcome unknown | Back off exponentially without polling before the interval |

Authorization polling is a sequence of ordinary requests. Do not hold one open,
use a WebSocket, or try to receive credentials through a callback.

## Store and rotate credentials

A successful device poll or browser exchange returns:

```json
{
  "accessToken": "ia1.…",
  "accessTokenExpiresAt": "2026-08-22T18:30:00Z",
  "refreshToken": "ir1.…",
  "instance": {
    "id": "…",
    "scopes": ["asset:receive"]
  }
}
```

Treat the token strings as opaque. An access token lasts 15 minutes. Send it in
the authorization header, never a query parameter:

```http
Authorization: Bearer ia1.…
```

Refresh shortly before access expiry, allowing for clock skew:

```http
POST /api/v1/link/refresh
Content-Type: application/json

{"refreshToken":"ir1.…"}
```

Only one worker may refresh an installation at a time. On success, durably
replace the old refresh token before releasing the new access token to other
workers. The old refresh token is spent when the server commits, even if the
response is lost. If the outcome is unknown, do not blindly retry the old token;
stop the installation and ask the owner to link again.

Replay of a replaced refresh token retained in Illarin's 90-day detection window
revokes the whole instance and all its access tokens. An older replacement is
still rejected, but Illarin no longer keeps enough information to attribute it
to an instance. A refresh family also expires after 90 days without authenticated
use. After a `401` from an ordinary access endpoint, serialize one refresh and
retry the original request once. A `401` from the refresh endpoint is terminal:
stop polling, delivery, and sync, remove local credentials, and offer to link
again. Never create a refresh-and-retry loop.

Secret-bearing responses use `Cache-Control: no-store`. A conforming platform
must apply the same policy to its own HTTP cache and diagnostic output.

## Update interoperability after an application release

An installation can replace its non-authoritative declaration without relinking:

```http
PUT /api/v1/instances/me
Authorization: Bearer ia1.…
Content-Type: application/json

{
  "applicationVersion": "4.3.0",
  "protocolVersion": 1,
  "capabilities": ["org.example.paperlantern:media-sidecars"],
  "acceptedTargets": ["example_bundle_v2", "example_bundle_v1"]
}
```

Send the complete replacement, not a patch. Names and granted scopes cannot be
changed here. If an upgrade changes either, keep the existing authorization or
ask the owner to revoke and link again; never silently widen access.

## Add support for a new platform or asset format

The linked-instance protocol has no platform switch statement. A platform may
use its own application name, installation name, namespaced capabilities, and
ordered target list without pretending to be another product.

There are two different extension jobs:

1. **The application already reads an Illarin export.** Declare those existing
   target IDs in preference order. No Illarin-specific branding or registration
   is needed.
2. **The platform needs a new file format.** Add a format module to Illarin. A
   declaration alone cannot upload a writer or make unknown bytes safe.

An Illarin format-module contribution should:

- Choose a stable lowercase module ID that can also be an export target.
- Declare its kind, read/write directions, recognition rules, role support,
  content limits, preservation namespace, and tested source formats.
- Implement the writer used for delivery and, when uploads use the format, a
  reader with fail-closed recognition.
- Preserve unknown data under the module's namespace instead of silently
  deleting it.
- Register through that kind's `Modules()` list; the server builds one registry
  from those lists.
- Add declaration, recognition, round-trip, cross-origin, size-limit, and corpus
  tests. Do not derive fixtures from production data.

Capabilities follow the same rule: the application may declare a namespaced
value freely, but Illarin must explicitly implement any behavior that consumes
it. Unknown values remain inert. This seam gives platform developers room to add
support without granting remote code or remote branding control.

## Collect deliveries

An owner presses send on an asset page and Illarin queues the asset for one of
their installations. Illarin never calls out, so collection is a pull. It needs
the `asset:receive` scope.

```http
POST /api/v1/deliveries/collect
Authorization: Bearer ia1.…
Content-Type: application/json

{"acknowledge":["<delivery-id>","<delivery-id>"]}
```

Illarin holds the request for 25 to 30 seconds. It answers `200` as soon as
there is work and `204` when the wait ends with nothing queued. Send
`"acknowledge": []` when there is nothing to confirm; the field is required.

This is a durable queue read, not authorization polling, and the two never share
a request. Illarin checks the credential, the `asset:receive` scope and the
asset's own visibility again at the moment work is released, so an asset
withdrawn after it was queued never arrives.

A `200` carries one entry per released delivery:

```json
{
  "deliveries": [
    {
      "id": "…",
      "assetId": "…",
      "contentGeneration": 4,
      "kind": "character",
      "name": "…",
      "format": "example_bundle_v2",
      "label": "Example bundle",
      "queuedAt": "2026-08-23T18:30:00Z",
      "leaseExpiresAt": "2026-08-23T18:45:00Z",
      "artifacts": [
        {"kind": "export", "url": "https://…/delivery/…/export?expires=…&signature=…"},
        {"kind": "picture", "url": "https://…/media/…", "mediaId": "…",
         "role": "expression", "isCover": false}
      ]
    }
  ]
}
```

`format` is the first target in your declared order that Illarin can write for
that asset, or `raw` for the creator's own uploaded file when nothing else fits.
The addresses are short-lived and signed: fetch them with ordinary `GET`s, which
makes a large file retryable rather than an all-or-nothing read. Every image the
asset holds is listed, so a format that cannot carry one can still be installed
with it; a format that embeds an image hands you those bytes twice.

Rules for a conforming client:

- Delivery is at least once. Deduplicate on `id`, install idempotently, and
  acknowledge only after the work is durably stored.
- An unacknowledged delivery comes back when its lease runs out. After a few
  unacknowledged takes Illarin stops offering it, so acknowledge what you install.
- Open one wait at a time. A second request supersedes the first, which then
  answers `204`; two workers waiting for the same installation simply take turns.
- After a failure, back off exponentially with jitter and honour `Retry-After`.
  `429` is a rate limit and `503` means Illarin is holding as many waits as it will.
- Store `contentGeneration` against `assetId`. A larger one later means the file
  changed.

## Report your library

`library:sync` mirrors what an installation holds so the site can show its owner
what is installed and what has moved on since. It is outbound only: Illarin never
writes this and never sends it back to another installation.

```http
POST /api/v1/library/sync
Authorization: Bearer ia1.…
Content-Type: application/json

{
  "snapshot": false,
  "entries": [{"assetId": "…", "contentGeneration": 4}],
  "removed": ["…"]
}
```

Set `snapshot` to `true` to replace the whole mirror for this installation;
anything absent is removed, so a snapshot may not also carry `removed`. Leave it
`false` to add, update and remove only what you name. At most 2000 entries and
2000 removals per request, and at most 256 KiB of body.

Leave `contentGeneration` out when an installation predates the counter. Illarin
records the asset's current generation rather than calling the install out of
date: an installation that cannot say which version it holds has not told us it
is behind.

Report immutable asset ids and never addresses, in both directions, so a creator
renaming something cannot break your state. Send incremental reports as things
change and a full snapshot occasionally, so a missed update repairs itself.
The response counts what was recorded:

```json
{"accepted": 142, "removed": 3, "ignored": 1}
```

`ignored` counts entries naming an asset Illarin cannot offer, such as one that
has since been deleted.

## Revocation and multiple instances

The account settings page lists and revokes installations independently.
Revoking one invalidates both of its credential classes immediately, wipes its
declaration, and deletes its pending deliveries and its library mirror. Another
installation on the same account keeps all three.

Your application should provide a local unlink action too. Until a public remote
revocation endpoint is specified, local unlink removes local credentials and
tells the owner to revoke the matching instance in Illarin settings. Match it by
the server-issued instance ID and the displayed application/installation names,
not by token prefix alone.

## Conformance checklist

Before calling an integration complete, verify all of these:

- Browser authorization uses the system browser, S256 PKCE, random state, and an
  exact literal loopback callback.
- The callback rejects a wrong state, wrong path, missing code, denial, timeout,
  and duplicate callback.
- Device fallback is manual, shows no prefilled link, obeys the persistent
  `slow_down` interval, and stops on every terminal response.
- Required arrays are sent even when empty; declarations stay below every bound.
- The application requests only the scopes it uses.
- Credentials are isolated per installation, stored securely, redacted from
  logs, and never placed in a URL.
- Refresh is serialized and the replacement is committed atomically.
- An access-endpoint `401` causes at most one refresh and retry; a refresh-endpoint
  `401` stops every background worker and removes unusable credentials.
- Updating a declaration cannot change names or scopes.
- Two installations of the same application can link, refresh, update, and
  unlink without sharing state.
- Unknown capabilities and targets produce no privileged behavior.
- One delivery wait is open at a time, `204` is handled, artifacts are fetched
  as ordinary retryable `GET`s, and deliveries are acknowledged only after they
  are durably installed.
- Delivery ids are deduplicated, so the same delivery arriving twice installs once.
- Library reports name immutable asset ids, stay inside every bound, and a
  snapshot carries no removals.
- All tests use synthetic accounts, names, codes, and assets.

For exact schemas, error bodies, and status codes, use `/openapi.yaml` as the
source of truth.
