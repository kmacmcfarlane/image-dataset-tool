# Sampling During Training

Generate sample images/videos during training by preparing a prompt file.

## Warning

Sampling consumes considerable VRAM. Be careful with large frame counts. Generation takes time, so adjust frequency accordingly.

## Training Command Options

Add these to your training command:

```bash
--vae path/to/vae \
--vae_chunk_size 32 --vae_spatial_tile_sample_min_size 128 \
--text_encoder1 path/to/text_encoder1 \
--text_encoder2 path/to/text_encoder2 \
--sample_prompts /path/to/prompt_file.txt \
--sample_every_n_epochs 1 \
--sample_every_n_steps 1000 \
--sample_at_first
```

- `--sample_prompts`: Path to prompt file
- `--sample_every_n_epochs`: Generate every N epochs
- `--sample_every_n_steps`: Generate every N steps
- `--sample_at_first`: Generate at training start

Output saved in `sample/` directory within `--output_dir`. Still images as `.png`, videos as `.mp4`.

## Prompt File Format

Text file with one prompt per line. Lines starting with `#` are comments.

```
# prompt 1: cat video
A cat walks on the grass, realistic style. --w 640 --h 480 --f 25 --d 1 --s 20

# prompt 2: dog image
A dog runs on the beach, realistic style. --w 960 --h 544 --f 1 --d 2 --s 20
```

## Prompt Options

| Option | Description | Default |
|--------|-------------|---------|
| `--w` | Width | 256 |
| `--h` | Height | 256 |
| `--f` | Frame count (1 = still image) | 1 |
| `--d` | Seed | random |
| `--s` | Generation steps | 20 |
| `--g` | Embedded guidance scale | Model-dependent (HV: 6.0, FramePack: 10.0, Kontext: 2.5) |
| `--fs` | Discrete flow shift | 14.5 for 20 steps |

### Model-Specific Options

**I2V models:**
```
prompt text --i path/to/image.png --w 832 --h 480 --f 25 --d 1 --s 20
```

**Wan2.1 Fun Control:**
```
prompt text --cn path/to/control_video_or_dir --w 832 --h 480 --f 25 --d 1 --s 20
```

**CFG models (Wan2.1, SkyReels):**
```
prompt text --n negative prompt text --l 6.0 --w 832 --h 480 --f 25 --d 1 --s 20
```

- `--n`: Negative prompt
- `--l`: Guidance scale (6.0 for SkyReels V1; 5.0 for Wan2.1)

**Control image models (FramePack, FLUX.1 Kontext):**
```
prompt text --ci path/to/control_image.jpg --w 1024 --h 1024 --d 1 --s 25
```

### Qwen-Image-Layered

- `--f` specifies output layer count
- Prompts optional (Qwen2.5-VL generates from control images if omitted)
- Total output = `--f` + 1 (original + separated layers)

## Flow Shift Recommendations

| Model | Steps | `--fs` |
|-------|-------|--------|
| HunyuanVideo | 50 | 7.0 |
| HunyuanVideo | <20 | 17.0 |
| FramePack | default | 10.0 |
| FLUX.1 Kontext | default | 0 (flux_shift) |
