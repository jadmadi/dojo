---
name: data-pipeline
description: Build and operate a batch data pipeline with idempotent writes and backfills.
---

# Data Pipeline

Use this skill when adding a stage, changing a schema, or investigating a
pipeline run that produced wrong or missing data.

## Quick Start

Define the source, the transform, and the destination. Run the pipeline for one
partition locally before scheduling it. Confirm the row counts at each stage and
compare a sample against the source.

## Idempotency

Every write is keyed and upserted. A rerun of the same partition must produce
the same rows. Delete the target partition before writing when the transform is
a full rebuild, and never append without a key. If a stage cannot be idempotent,
document why and make the failure explicit.

## Schemas

Schemas are versioned and additive. Add nullable columns, never rename or drop
in place. Consumers read the version they declare. When an incompatible change
is unavoidable, publish a new table and migrate readers before removing the old
one.

## Backfills

Backfills run from a separate job, in bounded batches, with a rate limit.
Record the partition range, the start time, and the row count. A backfill that
touches production readers runs off-peak and pauses if error rate rises.

## Monitoring

Every run reports duration, rows in, rows out, and rejected rows. Alert on three
conditions: a failed run, a run that produced zero rows, and a run that is more
than twice as slow as the trailing median. Alerts name the stage and partition.

## Debugging

Reproduce with one partition. Compare the source snapshot, the intermediate
table, and the destination. Check the schema version on each side before
reading the transform. A mismatch between producer and consumer schemas explains
most silent data loss.

## Cost

Partition by date, cluster by the highest-cardinality filter used by readers,
and avoid selecting columns the job does not use. Review bytes scanned after
any transform change.
