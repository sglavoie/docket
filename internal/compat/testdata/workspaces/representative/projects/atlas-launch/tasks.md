# Project: Atlas Launch
<!-- tt-schema: 19 -->
<!-- PROJECT-COMMENTS -->
<!-- project-comment: 3c9b2f4e-5d6a-4b7c-8d9e-0f1a2b3c4d01 2026-09-02T10:00:00Z -->
Kickoff agreed: beta before October.
<!-- project-comment: 3c9b2f4e-5d6a-4b7c-8d9e-0f1a2b3c4d02 2026-09-03T11:00:00Z -->
Signing certificate is the long pole.
Track it weekly.
<!-- /PROJECT-COMMENTS -->

## Task: Ship beta build
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000101, short_id: 1, state: doing, priority: 1, tags: feature;release, created: 2026-09-04T09:00:00Z, updated: 2026-09-04T09:00:00Z, state_changed: 2026-09-04T09:00:00Z, due: 2026-10-09, pinned: true, pinned_at: 2026-09-04T10:00:00Z, attachments: file|release-checklist.txt|%2Ftmp%2Frelease-checklist.txt|0|2026-09-05T13:00:00Z;url|https%3A%2F%2Fexample.com%2Fatlas%2Frelease|https%3A%2F%2Fexample.com%2Fatlas%2Frelease|0|2026-09-05T14:00:00Z;code|cmd%2Frelease.go%3A42|cmd%2Frelease.go|42|2026-09-05T15:00:00Z, context: laptop, due_time: 17:30, depends: 6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a02:0a7e5d10-2c3b-4f6a-8e9d-000000000201, check_in: 2026-10-08, check_in_repeat: 7, reviewed: 2026-09-30T16:00:00Z -->
<!-- RESUME -->
Waiting on notarization log; rerun step 3.
<!-- /RESUME -->
<!-- DESCRIPTION -->
Cut the release branch.

- build
- notarize
- upload
<!-- /DESCRIPTION -->
<!-- COMMENTS -->
<!-- comment: 2026-09-05T12:00:00Z -->
Branch cut from main.
<!-- comment: 2026-09-06T12:00:00Z -->
Notarization failed once; retrying.
<!-- /COMMENTS -->

### Task: Write release notes
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000102, short_id: 2, state: backlog, priority: 2, parent: 0a7e5d10-2c3b-4f6a-8e9d-000000000101, created: 2026-09-05T09:00:00Z, updated: 2026-09-05T09:00:00Z, state_changed: 2026-09-05T09:00:00Z, deferred: 2026-10-07T09:00:00Z, checklist: 1|Gather+changelog;0|Draft+summary, defer_count: 2 -->

#### Task: Proofread release notes
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000103, short_id: 3, state: inbox, priority: 0, parent: 0a7e5d10-2c3b-4f6a-8e9d-000000000102, created: 2026-09-06T09:00:00Z, updated: 2026-09-06T09:00:00Z, state_changed: 2026-09-06T09:00:00Z, review: true -->

#### Task: Collect screenshots
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000104, short_id: 4, state: backlog, priority: 0, parent: 0a7e5d10-2c3b-4f6a-8e9d-000000000102, created: 2026-09-06T09:00:00Z, updated: 2026-09-06T09:00:00Z, state_changed: 2026-09-06T09:00:00Z -->
<!-- checklist-origin: {"project_id":"6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01","task_id":"0a7e5d10-2c3b-4f6a-8e9d-000000000102","index":1} -->

## Task: Await signing certificate
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000105, short_id: 5, state: waiting, priority: 0, tags: question, created: 2026-09-07T09:00:00Z, updated: 2026-09-07T09:00:00Z, state_changed: 2026-09-07T09:00:00Z, due: 2026-10-12, check_in: 2026-10-10 -->

## Task: Localize installer
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000106, short_id: 6, state: paused, priority: 0, created: 2026-09-08T09:00:00Z, updated: 2026-09-08T09:00:00Z, state_changed: 2026-09-08T09:00:00Z -->

## Task: Dark mode icons
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000107, short_id: 7, state: someday, priority: 0, tags: idea, created: 2026-09-08T09:00:00Z, updated: 2026-09-08T09:00:00Z, state_changed: 2026-09-08T09:00:00Z -->

## Task: Set up CI signing
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000108, short_id: 8, state: done, priority: 0, created: 2026-09-09T09:00:00Z, updated: 2026-09-09T09:00:00Z, state_changed: 2026-09-09T09:00:00Z, completed: 2026-09-09T09:00:00Z, reviewed: 2026-09-08T15:00:00Z -->

## Task: Old updater shim
<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000109, short_id: 9, state: canceled, priority: 0, created: 2026-09-09T09:00:00Z, updated: 2026-09-09T09:00:00Z, state_changed: 2026-09-09T09:00:00Z, completed: 2026-09-09T09:00:00Z -->
