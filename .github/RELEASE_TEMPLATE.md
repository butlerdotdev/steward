## Steward ${TAG}

### Installation

#### Container Image
```bash
docker pull ghcr.io/butlerdotdev/steward:${TAG}
```

#### Helm Chart
```bash
helm repo add butlerlabs https://butlerdotdev.github.io/charts
helm repo update
helm install steward butlerlabs/steward --version ${TAG}
```

### What's Changed

<!-- Release notes will be auto-generated or manually added here -->

### Upgrading

See the [upgrade guide](https://github.com/butlerdotdev/steward/blob/main/docs/upgrade.md) for migration instructions.

---

*Steward is a community-governed fork of [Kamaji](https://github.com/clastix/kamaji), providing stable releases under the Apache 2.0 license.*
