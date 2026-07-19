# Request Debug Logging Design

## Goal

Add an admin-only debug snapshot for relay requests so local operators can compare:

- the downstream JSON request received from the client
- the final upstream JSON request sent to the selected channel

This is intended for diagnosing request failures caused by parameter mapping, disabled-field removal, pass-through behavior, model mapping, or parameter override issues.

The implementation must also remain easy to carry on top of upstream new-api updates. This is a personal/local customization, so the patch should minimize long-term merge friction.

## Upstream Sync Constraints

The first implementation should be shaped as a small, easy-to-reapply local patch:

- avoid database schema migrations in the first version
- avoid frontend changes in the first version
- keep the feature disabled by default
- keep configuration in one backend setting/env area rather than scattering channel-specific switches
- keep redaction, truncation, and snapshot assembly in one helper package or relay-common helper
- touch the smallest practical number of relay entry points
- do not modify provider-specific adaptors unless a path cannot be captured from the common relay flow
- prefer appending `Other.admin_info.request_debug` over changing log model fields

When upstream changes are pulled in, this design should let the local patch be replayed by resolving conflicts in a few predictable files rather than across dozens of channel adaptors.

The implementation should be kept as one or a small number of focused commits. That makes it easier to maintain a personal branch, rebase onto upstream, or cherry-pick the local customization after upgrading.

## Scope

The first version covers JSON-based relay paths where the code already produces a final upstream JSON payload before dispatch:

- OpenAI-compatible text/chat relay through `TextHelper`
- OpenAI Responses relay
- Claude/Gemini compatible JSON relay where the final upstream request is marshaled before `DoRequest`

The first version does not store multipart/audio/task binary bodies. Those paths may record metadata only, such as content type, size, and an unsupported-body marker.

## Configuration

The feature is disabled by default.

Configuration should support:

- `off`: no request debug snapshots are recorded
- `error_only`: snapshots are attached only when the relay request fails
- `always`: snapshots are attached to both success consume logs and error logs

The default max stored size per request side should be conservative, for example 16 KiB or 32 KiB. Oversized payloads are truncated and also include a full-body SHA-256 digest.

## Data Model

Store snapshots under the existing log field:

```json
{
  "admin_info": {
    "request_debug": {
      "mode": "always",
      "request_path": "/v1/chat/completions",
      "relay_mode": 1,
      "content_type": "application/json",
      "downstream": {
        "size": 1234,
        "sha256": "hex",
        "truncated": false,
        "body": "{...}"
      },
      "upstream": {
        "size": 2345,
        "sha256": "hex",
        "truncated": true,
        "body": "{...truncated...}"
      }
    }
  }
}
```

This follows the existing `Other.admin_info` convention. Non-admin log views already strip `admin_info`, so request snapshots remain admin-only without adding a new database table or changing normal user log visibility.

## Capture Points

Downstream capture reads from `common.GetBodyStorage(c)`. This avoids consuming `c.Request.Body` again and works with the existing reusable body cache.

Upstream capture happens after the final JSON mutation steps:

1. adaptor conversion
2. disabled-field removal
3. parameter override
4. immediately before creating the outbound body reader

This makes the recorded upstream body match the payload that will actually be sent.

## Error Handling

For `always`, success logs receive the snapshot through the existing `Generate*OtherInfo` flow before `model.RecordConsumeLog`.

For `error_only`, the snapshot is retained in request context or `RelayInfo` during processing and attached only when an error log is recorded.

If snapshot capture itself fails, relay behavior must not fail. The log should include a small marker such as:

```json
{
  "request_debug_error": "failed to read downstream body: ..."
}
```

## Redaction

Snapshots must be sanitized before storing:

- redact secret-like JSON keys: `authorization`, `api_key`, `apikey`, `access_token`, `refresh_token`, `key`, `token`, `password`, `secret`
- redact credential-bearing headers if header capture is added later
- truncate large string values, especially base64 and data URLs
- for image/audio/file content, store metadata rather than raw content

Redaction should run before size truncation so secrets are not retained in the saved prefix.

## Testing

Backend tests should cover observable behavior:

- feature disabled means no `request_debug` appears in `Other`
- `always` attaches sanitized downstream and upstream snapshots to consume logs
- `error_only` skips successful logs and attaches snapshots to error logs
- final upstream snapshot is captured after disabled-field removal and parameter override
- oversized bodies are truncated and include SHA-256 digest and original size
- non-admin log formatting strips the nested `admin_info.request_debug`

Tests should use deterministic table cases and `testify/require` plus `testify/assert`.

## Open Implementation Notes

Prefer a small service or relay-common helper for building sanitized snapshots. Avoid scattering redaction and truncation logic across individual channel adaptors.

Do not add a new database column or table in the first version. The existing `logs.other` JSON field is sufficient for a personal debug workflow and avoids cross-database migration risk.

For local maintenance, document the touched files after implementation and keep a short manual verification command list. After pulling upstream changes, the expected recovery flow is:

1. rebase or merge upstream
2. resolve conflicts in the small set of touched files
3. run the focused request-debug tests
4. run the existing relay/log tests affected by the touched files
