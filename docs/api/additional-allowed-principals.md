# Additional VPC Endpoint Service Principals

Clusters can grant additional AWS IAM principals permission to connect to the
cluster's PrivateLink VPC endpoint service. Set
`spec.hostedCluster.platform.aws.additionalAllowedPrincipals` when creating a
cluster or updating it later:

```json
{
  "spec": {
    "hostedCluster": {
      "platform": {
        "aws": {
          "additionalAllowedPrincipals": [
            "arn:aws:iam::123456789012:role/hcp-client"
          ]
        }
      }
    }
  }
}
```

The list accepts up to 25 unique IAM ARNs, each no longer than 255 characters.
Supported resources are account principals (`:root`), roles, users, and
federated users. On update, the supplied list replaces the customer-managed
list. Supply an empty array (`[]`) to remove all customer-added principals.
HyperShift separately manages the platform's required principal; it is not
returned in this field and cannot be changed through the customer API.

After adding a principal, the customer must create an interface VPC endpoint in
the cross-account VPC and configure a Route 53 private hosted zone to direct
API Server and OAuth traffic through that endpoint. The API only updates
endpoint-service permissions; it does not create the endpoint or DNS resources.
