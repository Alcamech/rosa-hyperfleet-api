# Spot instances

Spot is configured on an AWS NodePool through the HyperShift passthrough. Set
`spec.nodePool.platform.aws.placement.marketType` to `Spot`; `spot.maxPrice` is
optional.

```json
{
  "spec": {
    "nodePool": {
      "platform": {
        "type": "AWS",
        "aws": {
          "placement": {
            "marketType": "Spot",
            "spot": { "maxPrice": "0.50" }
          }
        }
      }
    }
  }
}
```

Spot does not require a termination-handler queue. Without one, Spot uses simple
mode and HyperShift does not enable graceful drain on interruption. To enable
graceful handling, customers can provision an SQS queue and provide its URL at
`spec.hostedCluster.platform.aws.terminationHandlerQueueURL` on the Cluster.
The URL is optional, must be a valid regional SQS queue URL in the Cluster's AWS
region, and may be used by only one Cluster in the account. Seaplane validates
the URL and passes it to HyperShift; it does not create or manage the queue or
its EventBridge rules.
