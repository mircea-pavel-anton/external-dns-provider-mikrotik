# ExternalDNS Webhook Provider for Mikrotik

> [!IMPORTANT]
> While this software has reached version `v1.0.0`, it has not yet undergone extensive testing in large-scale, real-world environments. As such, it may still have bugs and may not yet be fully suitable for production use.
>
> I encourage users to report any issues or suggest improvements, as this project remains under active development. Thank you for contributing!

[ExternalDNS](https://github.com/kubernetes-sigs/external-dns) is a Kubernetes add-on for automatically managing DNS records for Kubernetes ingresses and services by using different DNS providers. This webhook provider allows you to automate DNS records from your Kubernetes clusters into your MikroTik router.

Supported DNS record types: `A`, `AAAA`, `CNAME`, `MX`, `NS`, `SRV`, `TXT`

For examples of creating DNS records either via CRDs or via Ingress/Service annotations, check out the [`example/` directory](./example/).

## 🎯 Requirements

- ExternalDNS >= `v0.15.0`

> [!Note]
> `v0.15.0` of ExternalDNS added support for `providerSpecific` annotations in Ingress/Service objects for webhook providers.
>
> While older versions of ExternalDNS may work, support for this feature will not be present.

- Mikrotik RouterOS (tested on `7.16` stable)

> [!Note]
> While other versions of RouterOS **might** work, they have not been officially tested. If you are using this webhook successfully with a different ROS version, feel free to post a comment in mirceanton/external-dns-provider-mikrotik#141
>
> Thus far, we know for sure `7.16` works and `7.12` does not.

## 🚫 Limitations

### Multiple `Targets`

Currently, `DNSEndpoints` with multiple `targets` are **not** supported. No error will be thrown, but only one record will be created with the first target from the list.

This means that when creating a `DNSEndpoint` like this, only the first of the two targets will be taken into account (i.e. `192.192.192.192`).

```yaml
---
apiVersion: externaldns.k8s.io/v1alpha1
kind: DNSEndpoint
metadata:
  name: a-record
spec:
  endpoints:
    - dnsName: a.example.com
      recordTTL: 3605
      recordType: A
      targets:
        - 192.192.192.192
        - 193.193.193.193

```

The problem is that the External DNS controller will detect a drift on this and it will continuously attempt to update the DNS record, thus it will constantly send `PUT` requests to your RouterOS instance on every reconciliation loop.

### Regexp Records

While the webhook can read records with a regexp defined, external-dns itself cannot manage them. This means that they either need to be excluded via `domainFilters` or `excludeDomains` so that external-dns will not try to assume ownership over them.

The problem is that from Mikrotiks perspective, a DNS record can **either** have a `name` or a `regexp`. They are mutually exclusive.

This is problematic because, even though we can create an `Endpoint` with no name, external-dns will try to create a TXT record to keep track of the ownership over said record. If the main record has no name, it errors out creating the TXT record too, since the TXT record name is based on the name of the main record.

See mirceanton/external-dns-provider-mikrotik#166

### Multiple provider-specific annotations

In the case of multiple external-dns instances, each with a different provider (for example this one and the cloudflare one), there are problems with passing in annotations for provider-specific configuration. Due to a bug in the upstream external-dns, all annotations will be passed as provider-configuration.

This will cause the webhook to complain that invalid provider-specific configuration entries have been passed and error out. While this check can be removed, it will cause external-dns to continuously detect a drift between the Endpoint and the DNS records in RouterOS, thus attempting a new reconcile at every loop. This is also not desired.

See mirceanton/external-dns-provider-mikrotik#140 and kubernetes-sigs/external-dns#4951

## ⚙️ Configuration Options

### MikroTik Configuration

| Environment Variable        | Description                                                                        | Default Value |
|-----------------------------|------------------------------------------------------------------------------------|---------------|
| `MIKROTIK_BASEURL`          | URL at which the RouterOS API is available. (ex. `https://192.168.88.1:443`)       | N/A           |
| `MIKROTIK_USERNAME`         | Username for the RouterOS API authentication.                                      | N/A           |
| `MIKROTIK_PASSWORD`         | Password for the RouterOS API authentication.                                      | N/A           |
| `MIKROTIK_SKIP_TLS_VERIFY`  | Whether to skip TLS verification (true or false).                                  | `false`       |
| `LOG_FORMAT` | The format in which logs will be printed. (`text` or `json`)                       | `json`        |
| `LOG_LEVEL`                 | The verbosity at which logs are printed logs. (`debug`, `info`, `warn` or `error`) | `info`        |
| `MIKROTIK_DEFAULT_TTL`      | Default TTL value to be set for DNS records with no specified TTL.                 | `3600`        |

### Webhook Server Configuration

| Environment Variable             | Description                                                      | Default Value |
|----------------------------------|------------------------------------------------------------------|---------------|
| `SERVER_HOST`                    | The host address where the server listens.                       | `localhost`   |
| `SERVER_PORT`                    | The port where the server listens.                               | `8888`        |
| `SERVER_READ_TIMEOUT`            | Duration the server waits before timing out on read operations.  | N/A           |
| `SERVER_WRITE_TIMEOUT`           | Duration the server waits before timing out on write operations. | N/A           |
| `DOMAIN_FILTER`                  | List of domains to include in the filter.                        | Empty         |
| `EXCLUDE_DOMAIN_FILTER`          | List of domains to exclude from filtering.                       | Empty         |
| `REGEXP_DOMAIN_FILTER`           | Regular expression for filtering domains.                        | Empty         |
| `REGEXP_DOMAIN_FILTER_EXCLUSION` | Regular expression for excluding domains from the filter.        | Empty         |

## 🚀 Deployment

1. Create a service account in RouterOS. This local user needs read and write access to manage static DNS.
2. Create a Kubernetes namespace for your External DNS deployment

    ```yaml
    ---
    apiVersion: v1
    kind: Namespace
    metadata:
      name: external-dns
    ```

3. Create a Kubernetes secret with the connection details for your RouterOS instance:

    ```yaml
    ---
    apiVersion: v1
    kind: Secret
    metadata:
      name: mikrotik-credentials
      namespace: external-dns
    stringData:
      MIKROTIK_BASEURL: "https://192.168.88.1:443"
      MIKROTIK_USERNAME: "external-dns"
      MIKROTIK_PASSWORD: "external-dns"
      MIKROTIK_SKIP_TLS_VERIFY: "true"
    ```

4. Add the External DNS helm repository and update your local cache

    ```bash
    helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
    helm repo update
    ```

5. Configure your helm values. Take a look at the [example values.yaml](./example/values.yaml)
6. Install the External DNS helm chart

    ```bash
    helm upgrade --install --namespace external-dns external-dns external-dns/external-dns -f values.yaml
    ```

> [!TIP]
> By default, support for MX, NS and SRV records is disabled and needs to be enabled via the `--managed-record-types` argument.
> Make sure to set `--managed-record-types=SRV` if you want to enable SRV records, and so on.

## ⭐ Stargazers

[![Star History Chart](https://api.star-history.com/svg?repos=mirceanton/external-dns-mikrotik-webhook&type=Date)](https://star-history.com/#mirceanton/external-dns-mikrotik-webhook&Date)

---

## 🤝 Gratitude and Thanks

Thanks to all the people who donate their time to the [Home Operations](https://discord.gg/home-operations) Discord community.
