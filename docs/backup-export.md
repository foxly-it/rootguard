# Encrypted full backup and clean-install restore

RootGuard's dedicated Backups page can download a portable full backup protected by an
operator-chosen passphrase. The binary file uses the interoperable age-v1
format with scrypt passphrase encryption. RootGuard never stores the
passphrase; losing it makes the archive unrecoverable.

## Included data

- RootGuard's logical Unbound state, custom configuration, and history;
- RootGuard's AdGuard credentials/state and blockpage authentication state;
- generated installation state;
- live AdGuard Home configuration and work data;
- live Unbound runtime state;
- `manifest.json`, with schema version, creation time, and SHA-256 plus size
  for every regular payload file.

Browser sessions, external `.env` secrets, internal pre-update restore points,
cleanup history, and temporary export files are excluded. Source paths and
container names are fixed in Core and cannot be selected by a browser request.
Symlinks and non-regular files abort the export.

## Transport and staging

The final download is encrypted, including its file names and manifest. Docker
copies and local state are staged briefly as plaintext in a fresh mode-`0700`
directory inside Core's protected data volume. The directory is removed after
success, failure, or request cancellation. AdGuard/Unbound updates cannot run
at the same time.

When RootGuard is opened over plain HTTP, the passphrase itself has no HTTPS
transport protection between browser and local WebApp. Prefer the supported
reverse-proxy setup in [https-reverse-proxy.md](https-reverse-proxy.md), or use
the feature only on a trusted local management network.

## Guided clean-install restore

The same Backups page accepts an encrypted export for recovery onto a fresh
RootGuard control-plane installation. Preview decrypts into private staging
and validates the schema, required files, allowlisted roots, regular file
types, exact manifest inventory, sizes, and SHA-256 checksums. Upload size,
expanded size, and entry count are bounded. Preview never changes persistent
state and reports the archived installation settings plus clean-target
preflight checks.

The DNS bind address and port may be changed for a replacement host and must
then pass another preview. Apply uploads and validates the archive again,
requires explicit confirmation, and refuses a target with an installed stack
or any conflicting managed container, internal volume, or DNS network.
RootGuard creates the target containers in stopped state, restores the fixed
local and service paths, normalizes Unbound volume ownership, starts the stack,
and verifies the protected AdGuard-to-Unbound path. On failure it removes the
new managed Docker resources and restores the local volume contents captured
immediately before apply. Plaintext staging and rollback copies are removed on
every exit; the passphrase is neither logged nor persisted.

This workflow is for a clean replacement installation, not an in-place import
over a running appliance. Transactional live snapshots and post-update restore
verification remain separate 0.4 work.

## Command-line decryption

Install the standard `age` CLI, then decrypt and inspect the archive:

```sh
age --decrypt -o rootguard-backup.tar.gz rootguard-backup-YYYY-MM-DD.tar.gz.age
tar -tzf rootguard-backup.tar.gz
```

`age` prompts for the export passphrase. Keep the encrypted original until the
subsequent guided restore has been verified. Manual extraction does not itself
activate a RootGuard configuration.
