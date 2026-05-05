# File & Manifest Schemas

## Overview

The filesystem is the source of truth. SQLite mirrors manifest data as a queryable
index. If the DB is lost, the F-005 reconciler rebuilds it by scanning manifests.

All paths below are relative to `$DATA_DIR` (default `~/image-dataset-tool`).

**Portability**: `$DATA_DIR` can be copied to another machine and the app will work.
The `nats/` subdirectory contains ephemeral pipeline state and can be safely deleted —
it will be recreated on startup. The `secret.key` file must also be copied for
encrypted secrets to remain accessible.

## Directory structure

```
$DATA_DIR/
├── secret.key                       # AES-256-GCM key (mode 0600)
├── db.sqlite
├── nats/                            # JetStream file-backed storage
├── tmp/                             # staging for in-flight fetches
│   └── <job-id>/                    # cleaned up after processing
├── projects/
│   └── <project-slug>/
│       ├── manifest.json
│       └── subjects/
│           └── <subject-slug>/
│               ├── manifest.json
│               └── samples/
│                   ├── <sample-id>.<ext>
│                   ├── <sample-id>_thumb.jpg
│                   └── <sample-id>.json
└── exports/
    └── <project-slug>/<subject-slug>/<format>-<timestamp>/
```

## Project manifest

Path: `projects/<project-slug>/manifest.json`

```json
{
  "id": "uuid",
  "slug": "my-project",
  "name": "My Project",
  "description": "Optional description",
  "created_at": "2026-05-01T12:00:00Z",
  "updated_at": "2026-05-01T12:00:00Z"
}
```

## Subject manifest

Path: `projects/<project-slug>/subjects/<subject-slug>/manifest.json`

```json
{
  "id": "uuid",
  "project_id": "uuid",
  "slug": "subject-name",
  "name": "Subject Name",
  "linked_accounts": ["account-uuid-1", "account-uuid-2"],
  "created_at": "2026-05-01T12:00:00Z",
  "updated_at": "2026-05-01T12:00:00Z"
}
```

Note: account credentials (cookies) are NOT stored in manifests — they live only
in the encrypted SQLite `accounts` table. Manifests store only account UUIDs for
the linking relationship.

## Sample metadata

Path: `projects/<project-slug>/subjects/<subject-slug>/samples/<sample-id>.json`

```json
{
  "id": "uuid",
  "source": {
    "platform": "instagram",
    "post_id": "ABC123",
    "slide_index": 0,
    "url": "https://...",
    "taken_at": "2026-04-15T08:30:00Z"
  },
  "file": {
    "path": "<sample-id>.jpg",
    "sha256": "hex-encoded",
    "phash": "hex-encoded",
    "width": 1080,
    "height": 1350
  },
  "status": "kept",
  "is_duplicate": false,
  "duplicate_of": [],
  "edits": {
    "rotation_deg": 0.0,
    "crop_box": null,
    "auto_actions": []
  },
  "captions": {
    "natural-claude": {
      "study_id": "uuid",
      "text": "A woman standing on a beach at sunset...",
      "created_at": "2026-05-02T14:00:00Z"
    },
    "detailed-local": {
      "study_id": "uuid",
      "text": "...",
      "created_at": "2026-05-02T15:00:00Z"
    }
  },
  "fetched_at": "2026-05-01T12:00:00Z",
  "created_at": "2026-05-01T12:00:00Z",
  "updated_at": "2026-05-02T15:00:00Z"
}
```

The `captions` object is keyed by study slug. At export time, the exporter reads
the caption for the selected study slug.

## Thumbnail files

Path: `projects/<project-slug>/subjects/<subject-slug>/samples/<sample-id>_thumb.jpg`

- Format: JPEG (quality 80)
- Max dimension: configurable (default 768px on shortest edge)
- Generated during the `media.process` pipeline stage
- Regenerated if the configured thumbnail size changes (tracked in a metadata file
  or by comparing dimensions on read)

## Temp staging

Path: `tmp/<job-id>/<filename>`

Fetch workers download images here first. The `media.process` consumer then:
1. Computes sha256 + pHash
2. Checks for duplicates within the subject (sha256 exact + pHash Hamming distance)
3. Moves to final `samples/` path, generates thumbnail, writes manifest
4. If duplicate detected: file is still kept but flagged `is_duplicate = true` in
   SQLite and manifest (duplicate may be higher quality than existing sample)

The `tmp/<job-id>/` directory is cleaned up when all items in the job are processed.

## Export output

Path: `exports/<project-slug>/<subject-slug>/<format>-<timestamp>/`

Contents depend on the writer format. For kohya-ss:

```
<format>-<timestamp>/
├── <sample-id>.jpg      # edited image (rotation/crop applied)
└── <sample-id>.txt      # caption text from selected study
```
