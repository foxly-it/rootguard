# Clean install, same verification

Every release is verified with the same guarded end-to-end test on native Linux amd64/arm64 runners and Docker Desktop. It covers login, AIO deployment, recursive DNS resolution, and rejection of an invalid DNSSEC chain.

| Platform | Evidence |
| --- | --- |
| Linux amd64 | Native automated GitHub runner |
| Linux arm64 | Native automated GitHub runner |
| Docker Desktop | Apple Silicon / arm64 passed on 2026-07-28 |

[Open verification matrix and safe repeatable test ↗](https://github.com/foxly-it/rootguard/blob/main/docs/platform-support.md){: target="_blank" rel="noopener" }
