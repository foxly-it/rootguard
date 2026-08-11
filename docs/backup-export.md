# Encrypted full backup export

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

## Command-line decryption

Install the standard `age` CLI, then decrypt and inspect the archive:

```sh
age --decrypt -o rootguard-backup.tar.gz rootguard-backup-YYYY-MM-DD.tar.gz.age
tar -tzf rootguard-backup.tar.gz
```

`age` prompts for the export passphrase. Keep the encrypted original until the
subsequent restore has been verified. Guided clean-install restore is the next
0.4 roadmap item; manual extraction does not itself activate a RootGuard
configuration.
