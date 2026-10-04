# Deploying wherefolk

For the **Operator**. Everything the repository cannot do for you — the Tailscale admin console,
the secrets, seeding the first document — is here, in the order you need it. The design behind it
is in [ADR-0001](../adr/0001-tailscale-for-access.md),
[ADR-0006](../adr/0006-container-deployment-with-tailscale-sidecar.md) and
[ADR-0007](../adr/0007-configurable-port-fixed-interface.md).

What you end up with: `https://wherefolk.<tailnet>.ts.net`, reachable from the devices the ACL
names, with a real certificate, and from nowhere else.

## 1. Tailscale admin console (once)

1. **DNS → enable MagicDNS and HTTPS Certificates.** `tailscale serve` cannot obtain a certificate
   until both are on. The node's name will appear in Certificate Transparency logs; nothing
   reachable or revealing comes with it.
2. **Access controls → add the tag and the grant.** Merge this into your policy file, replacing the
   two addresses with the Operator's and the Editor's accounts (or tag/device names). `src` must be
   specific people or devices — never `*`:

   ```jsonc
   {
     "tagOwners": {
       "tag:wherefolk": ["autogroup:admin"]
     },
     "acls": [
       {
         "action": "accept",
         "src": ["operator@example.com", "editor@example.com"],
         "dst": ["tag:wherefolk:443"]
       }
     ],
     "tests": [
       {
         "src": "editor@example.com",
         "accept": ["tag:wherefolk:443"],
         "deny": ["tag:wherefolk:22", "tag:wherefolk:8080"]
       }
     ]
   }
   ```

   The `tests` block makes the console refuse a policy that stops granting the Editor access, or
   that grants more than 443. The grant must stay 443-only: the app also listens on 8080 over the
   shared loopback, and Serve's TLS is bypassed if that port is ever granted. Tailnet membership is otherwise coarse: without this grant every
   device you add to the tailnet later could reach the Directory.
3. **Funnel stays unavailable to the node.** Funnel needs a `funnel` entry in `nodeAttrs`. The
   default policy grants it to `autogroup:member`, and a tagged node is not a member — but check
   that no `nodeAttrs` entry targets `tag:wherefolk` or `*` with `"attr": ["funnel"]`. The Serve
   config also sets Funnel off explicitly; this is the second layer.
4. **Settings → OAuth clients → Generate.** Scope **Auth Keys: Write**, tag `tag:wherefolk`. Copy
   the client secret (`tskey-client-…`) now; it is shown once. Unlike an auth key it does not expire
   at 90 days.

## 2. Secrets (once)

The file consists of the three variables in [`deploy/.env.example`](../../deploy/.env.example):

```
TS_AUTHKEY=tskey-client-<secret>?ephemeral=false&preauthorized=true
WHEREFOLK_FULL_PASSPHRASE=<at least 8 characters, no surrounding whitespace>
WHEREFOLK_VERSION=latest
```

Keep `?ephemeral=false`: an ephemeral node is deleted when it goes offline and the Editor's
bookmark stops resolving. Single-quote the passphrase if it contains `$`. Leave it empty to run
without the Full tier.

Put the file under agenix in the Operator's NixOS configuration (a separate repository), for
example as a secret named `wherefolk-env` that the host decrypts to `/run/agenix/wherefolk-env`.
Compose then reads it in place and nothing secret is written into this checkout. If you use a plain
file instead, `deploy/.env` is already gitignored.

## 3. Seed the document (once)

The service refuses to start without `directory.json`, and the data volume starts empty. Place
the document in a new volume, owned by the service user (uid 65532) and mode 0600:

```bash
docker run --rm --user root --entrypoint install \
  -v wherefolk-data:/var/lib/wherefolk \
  -v "$PWD/directory.json:/seed/directory.json:ro" \
  ghcr.io/asphaltbuffet/wherefolk:latest \
  -o 65532 -g 65532 -m 0600 /seed/directory.json /var/lib/wherefolk/directory.json
```

Starting from nothing, use [`testdata/directory.json`](../../testdata/directory.json) as the seed
and replace its contents through the editor.

Compose may later log a warning that the volume was not created by Compose; that is expected and
harmless.

## 4. First start

Compose needs the env file on every subcommand, not just `up`, because it interpolates the whole file.

```bash
docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env pull
docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env up -d
```

`up` fails immediately and says so if `TS_AUTHKEY` is unset. Then check each claim:

| Check | Command | Expect |
|---|---|---|
| Sidecar joined the tailnet | `docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env exec tailscale tailscale status` | `wherefolk` listed, tagged `tag:wherefolk` |
| Serve is tailnet-only | `docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env exec tailscale tailscale serve status` | the URL, marked `(tailnet only)`, never `(Funnel on)` |
| App is up | `docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env logs wherefolk` | a `serving` line with `addr=127.0.0.1:8080`, no `startup failed` |
| Editor's path works | open `https://wherefolk.<tailnet>.ts.net/status` from the Editor's device | the status page, with a padlock and no warning |
| Everyone else is refused | open the same URL from a device the ACL does not name | connection times out |

If the node shows up as `wherefolk-1`, state was not persisted: confirm the
`wherefolk-tailscale-state` volume exists and was not removed by `docker compose down -v`. Never
use `-v` on this stack — it deletes the Directory.

## 5. Updating

```bash
docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env pull
docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env up -d
```

Deployment is always "pull and restart": the binary migrates its own data at startup ([high-level design §2.1](../design/high-level-design.md)).

## 6. Rolling back

Set `WHEREFOLK_VERSION` to the earlier tag in the secrets file and run `up -d`. If the newer
version had migrated the document to a newer schema, the older binary **refuses to start** rather
than truncating fields it does not understand — restore the document from a snapshot instead
(item 14 of the [high-level design](../design/high-level-design.md) (`docs/design/high-level-design.md`)
will take them nightly).

## 7. Debugging

The app always binds loopback, and no setting changes that. To look inside:

```bash
docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env logs -f wherefolk
docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env exec wherefolk sh
```

To reach the app from the host without going through the tailnet, publish a port **temporarily**
on the sidecar (the app shares its network namespace) in a throwaway override file, never in
`compose.yaml`, and remove it afterwards.

If the URL returns 502 after the sidecar restarted on its own, the app is still in the old network
namespace; restart it:

```bash
docker compose -f deploy/compose.yaml --env-file /run/agenix/wherefolk-env restart wherefolk
```

## Changing the port

Edit `WHEREFOLK_PORT` in `deploy/compose.yaml` **and** the `Proxy` target in
`deploy/tailscale-serve.json` together; the Serve file cannot read an environment variable.
`go test ./deploy/` checks both against the application's default and fails if you change only
one — update the tests if the change is deliberate.
