# Dataset Configuration

TOML configuration files for setting up image and video datasets in musubi-tuner.

## Critical Rules

- Each dataset **must** have a distinct `cache_directory` to prevent cache conflicts
- `cache_directory` is mandatory when using metadata JSONL files
- Video frame counts must be in N*4+1 format (5, 9, 13, 17, 21, 25...) for HunyuanVideo and Wan2.1

## Image Dataset with Caption Files

```toml
[[datasets]]
resolution = [1024, 1024]
caption_extension = ".txt"
batch_size = 1
enable_bucket = true
bucket_no_upscale = true
num_repeats = 1
cache_directory = "/path/to/cache"

  [[datasets.subsets]]
  image_directory = "/path/to/images"
```

Images paired with caption files sharing the same filename (e.g., `image1.jpg` + `image1.txt`).

## Image Dataset with JSONL Metadata

```toml
[[datasets]]
resolution = [1024, 1024]
batch_size = 1
enable_bucket = true
bucket_no_upscale = true
cache_directory = "/path/to/cache"

  [[datasets.subsets]]
  metadata_file = "/path/to/metadata.jsonl"
```

JSONL format:
```json
{"image_path": "/path/to/image1.jpg", "caption": "A cat sitting on a couch"}
{"image_path": "/path/to/image2.jpg", "caption": "A dog running in a park"}
```

## Video Dataset with Caption Files

```toml
[[datasets]]
resolution = [544, 960]
caption_extension = ".txt"
batch_size = 1
enable_bucket = true
bucket_no_upscale = true
target_frames = [1, 25, 45]
frame_extraction = "head"
cache_directory = "/path/to/cache"

  [[datasets.subsets]]
  video_directory = "/path/to/videos"
```

## Video Dataset with JSONL Metadata

JSONL format:
```json
{"video_path": "/path/to/video1.mp4", "caption": "A cat walking"}
{"video_path": "/path/to/frames_dir/", "caption": "A dog running"}
```

`video_path` can reference video files or directories of image sequences.

## Frame Extraction Methods

| Method | Description |
|--------|-------------|
| `head` | Extract first N frames |
| `chunk` | Split video into chunks of N frames |
| `slide` | Extract with specified stride |
| `uniform` | Sample uniformly across video |
| `full` | Extract all frames (trimmed to N*4+1) |

## Source FPS

```toml
source_fps = 30.0
```

When specified, frames are skipped to match model FPS (24 for HunyuanVideo, 16 for Wan2.1). Must be a decimal (e.g., `30.0` not `30`). Without `source_fps`, all frames are used.

## Control Images

### For Image Datasets

Control images enable one-frame FramePack training, FLUX.1 Kontext, FLUX.2, and Qwen-Image-Edit workflows.

```toml
  [[datasets.subsets]]
  image_directory = "/path/to/target_images"
  control_directory = "/path/to/control_images"
```

Target images = desired outputs; control images = starting points. Filenames must match.

Multiple control images use numbered suffixes: `image1_0.png`, `image1_1.png`.

### Control Image Resizing

- `no_resize_control = true`: Skip resizing, trim to architecture multiples (16px)
- `control_resolution = [1024, 1024]`: Resize with aspect ratio bucketing

### For Video Datasets

Control videos pair with target videos using matching filenames:

```toml
  [[datasets.subsets]]
  video_directory = "/path/to/target_videos"
  control_directory = "/path/to/control_videos"
```

## Qwen-Image-Layered

```toml
[[datasets]]
resolution = [1024, 1024]
batch_size = 1
multiple_target = true
cache_directory = "/path/to/cache"
```

Segmentation layer images stored alongside base images with numbered suffixes and alpha channels.

JSONL format supports `image_path_N` entries for layered operations.

## Multiple Datasets

Balance with `num_repeats`:

```toml
[[datasets]]
resolution = [1024, 1024]
batch_size = 1
num_repeats = 5

  [[datasets.subsets]]
  image_directory = "/path/to/small_dataset"

[[datasets]]
resolution = [1024, 1024]
batch_size = 1
num_repeats = 1

  [[datasets.subsets]]
  image_directory = "/path/to/large_dataset"
```

## FramePack-Specific

Use `frame_extraction = "full"` with large `max_frames` for longer content. Very long videos risk OOM during VAE encoding. Videos are trimmed to `N * latent_window_size * 4 + 1` frames.
