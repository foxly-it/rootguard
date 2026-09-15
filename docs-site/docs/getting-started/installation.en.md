# Start the control plane

!!! info "Automated installation"
    For the fast path with automatic Docker detection/installation, auto-generated security keys, and a short username/password prompt, see the [one-command quick start](https://rootguard.foxly.de/#quickstart) on the homepage. The steps below show every step individually for manual use.

```shell
mkdir rootguard && cd rootguard

curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v1.0.0/compose.release.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v1.0.0/.env.release.example

# Generate two independent security keys
openssl rand -hex 32
openssl rand -hex 32

# Fill in .env, then start RootGuard
docker compose -f compose.release.yaml up -d
```

Store two independently generated random values as `ROOTGUARD_API_TOKEN` and `ROOTGUARD_RECOVERY_TOKEN`, and set your own strong `ROOTGUARD_ADMIN_PASSWORD` in `.env`. RootGuard pulls versioned amd64/arm64 images from GHCR; no component checkout or local build is required. The control plane starts first and Setup then creates the DNS services.

```text
http://localhost:8080/login
```
