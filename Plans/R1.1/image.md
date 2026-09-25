# Catalogue image

Status: proposed, not built. Open choices are in [questions](QUESTIONS.md#open-questions).

### The image

Extend the [R1 container distribution](../R1/distribution.md#container-runtime) with the bundled service catalogue.

| | |
|---|---|
| **useful** | the catalogue ships **installed**, and `services.json` enables the reading half only — health, info, and the pair that watches the bus. Everything that acts on the box is installed and **not enabled**, which is a state the design already has and precisely what it is for ([runner § what an instance is](../R1/runner.md#what-an-instance-is)) |

Billing is **not in any stage**: it is designed and deferred
([future/billing.md](../R2.0/billing.md#billing-role--future)).
