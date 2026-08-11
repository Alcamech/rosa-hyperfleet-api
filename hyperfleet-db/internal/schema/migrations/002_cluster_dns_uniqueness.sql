-- DNS FQDNs include a hash4 slug (first 4 chars of internalId) to disambiguate
-- clusters sharing the same human-readable name across accounts. Two live
-- clusters with the same (name, hash4) would produce identical DNS records,
-- so we enforce uniqueness here as the atomic safety net behind the
-- platform-api's application-level collision check.
CREATE UNIQUE INDEX IF NOT EXISTS idx_cluster_name_hash4
    ON kubernetes_resources (name, (LEFT(spec->>'internalId', 4)))
    WHERE gvk = 'hyperfleet.io/v1alpha1/Cluster'
      AND deletion_timestamp IS NULL
      AND spec->>'internalId' IS NOT NULL;
