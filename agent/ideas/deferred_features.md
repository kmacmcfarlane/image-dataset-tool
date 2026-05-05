# Deferred Features

Features discussed during planning but explicitly deferred from MVP. Revisit after
core pipeline is working end-to-end.

## Encryption

- **Passphrase-derived key (Argon2id)**: Derive encryption key from a user passphrase
  prompted at startup instead of reading from a key file. Most secure option but
  requires typing a password each session. Implement as a config toggle alongside
  the file-based key (Option A). Use `golang.org/x/crypto/argon2`.

## Ingestion

- **Per-account ETA estimation**: Query IG API for total post count before starting
  a pull job. Allows ETA per account and for the entire job. May be expensive or
  unreliable depending on IG client approach — evaluate after IG-001 spike.
- **Video dataset support**: Download and process video from IG posts for video
  generation model training (e.g., HunyuanVideo, Wan). Requires different
  processing pipeline and storage strategy.
- **Additional sources**: TikTok, Twitter/X, Pinterest. Each gets its own
  `media.fetch.{provider}` subject and client implementation.

## Review & editing

- **Face-aware auto-cropping**: Detect faces in samples and offer smart crop
  suggestions centered on faces. Requires face detection model (Go or subprocess).
- **Similarity clustering**: Group near-duplicate samples visually for batch
  review. Uses pHash distance matrix + clustering algorithm.

## Captioning

- **Caption-prompt versioning**: Track prompt template changes over time within a
  study. Allow re-running captions with an updated prompt and comparing results.
- **Per-subject prompt libraries**: Curated prompt templates tuned for specific
  subject types (portraits, landscapes, products, etc.).

## Ingestion (continued)

- **Manual folder import**: Import a directory of existing images into a subject
  without going through a social media source. Publishes directly to
  `media.process`, skipping `media.fetch`. Common workflow for users who already
  have images collected manually.

## Review & editing (continued)

- **Caption text search**: Search/filter samples by caption content across studies.
  Requires FTS index on captions table.

## Export

- **Additional dataset writers**: diffusers format, OneTrainer format, custom
  user-defined layouts.
