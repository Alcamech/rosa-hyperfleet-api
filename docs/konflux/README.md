# Konflux

Documentation for Konflux CI/CD on this repository.

- [HyperFleet Konflux onboarding guide](https://github.com/openshift-online/rosa-hyperfleet/blob/main/docs/konflux-onboarding.md) — team checklist for onboarding new images (environment, Prow/Konflux split, MintMaker; status in [ROSAENG-59370](https://issues.redhat.com/browse/ROSAENG-59370))
- [Quay image tags and pinning](./quay-image-tags.md) — where images land in this repo, tag naming, and how to pick a build off `main`

**Konflux UI:** [`rosa-hyperfleet` application](https://konflux-ui.apps.kflux-prd-rh02.0fk9.p1.openshiftapps.com/ns/rosa-tenant/applications/rosa-hyperfleet/activity) · namespace `rosa-tenant` on `kflux-prd-rh02`

This repo is the **reference implementation** for `.tekton/` pipelines (`platform-api`, `hyperfleet-operator`).
