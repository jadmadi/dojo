---
name: incident-response
description: Run a production incident from detection to review with clear roles and evidence.
---

# Incident Response

Use this skill when a production service is failing, degraded, or returning
wrong results for users.

## First Five Minutes

Declare the incident in the incident channel, assign an incident lead, and state
the user-visible symptom in one sentence. Do not start with a theory. Open a
timeline document and record the detection time, the first symptom, and the
current impact.

## Roles

The incident lead coordinates and does not debug. One engineer investigates, one
communicates status, and one owns the rollback decision. On a small team, the
lead may also investigate, but the communication role stays separate so updates
do not stop.

## Communication

Update every fifteen minutes, even when nothing changed. State impact, current
action, and next update time. Customers get plain language and no speculation.
Internal updates include the hypothesis and the evidence for it.

## Mitigation Before Diagnosis

Stop the bleeding first. Roll back the last deploy, disable the failing feature
flag, or shift traffic. A rollback is the default mitigation until evidence
proves it cannot help. Do not debug forward on a system that is still failing
users.

## Evidence

Keep commands and outputs in the timeline. Record timestamps in UTC. Note what
was ruled out and why. Screenshots and log excerpts belong in the document, not
in chat history that will be lost.

## Resolution

Declare resolution only after the error rate and latency return to baseline for
at least thirty minutes. Confirm with a real user path, not only a health check.
Remove temporary mitigations deliberately, one at a time.

## Review

Write the review within two working days. Cover impact, timeline, root cause,
what went well, and three concrete actions with owners and dates. A review with
no owned actions is not finished.
