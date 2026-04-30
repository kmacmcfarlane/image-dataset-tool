# Wan 2.1 / 2.2

Text-to-video, image-to-video, text-to-image, and Fun Control generation with Wan2.1 and Wan2.2.

## Model Downloads

### Wan2.1

- **T5**: `models_t5_umt5-xxl-enc-bf16.pth`
- **CLIP**: `models_clip_open-clip-xlm-roberta-large-vit-huge-14.pth`
- **VAE**: `Wan2.1_VAE.pth` or `wan_2.1_vae.safetensors`
- **DiT**: From Comfy-Org repository (various sizes: 1.3B, 14B, etc.)
- **Fun Control**: Separate model weights available

### Wan2.2

- Same T5 as Wan2.1
- **No CLIP required**
- Same VAE as Wan2.1
- **Dual DiT models** (high and low noise)

## Available Tasks

Use `--task` to specify:

- `t2v-1.3B`, `t2v-14B`: Text-to-video
- `i2v-14B`: Image-to-video
- `t2i-14B`: Text-to-image
- `flf2v-14B`: First-last-frame to video
- Fun Control variants
- Wan2.2 variants (with dual noise models)

## Pre-caching

### Latent Pre-caching

```bash
python src/musubi_tuner/wan_cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/wan_vae.safetensors
```

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/wan_cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --t5 path/to/models_t5_umt5-xxl-enc-bf16.pth \
  --batch_size 16
```

- `--fp8_t5`: Run T5 in fp8 for VRAM savings

## Training

Script: `wan_train_network.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/wan_train_network.py \
  --task t2v-14B \
  --dit path/to/dit_model \
  --dataset_config path/to/toml \
  --sdpa --mixed_precision bf16 --fp8_base \
  --optimizer_type adamw8bit --learning_rate 2e-4 --gradient_checkpointing \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --network_module networks.lora_wan --network_dim 32 \
  --timestep_sampling shift --discrete_flow_shift 3.0 \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-lora
```

### Required Arguments

- `--task`: Model type
- `--network_module networks.lora_wan`
- `--timestep_sampling` and `--discrete_flow_shift`: Require experimentation

### Wan2.2 Timestep Ranges

| Model | `--min_timestep` | `--max_timestep` |
|-------|-----------------|-----------------|
| I2V low noise | 0 | 900 |
| I2V high noise | 900 | 1000 |
| T2V low noise | 0 | 875 |
| T2V high noise | 875 | 1000 |

### Memory Optimization

- `--fp8_base`: fp8 for DiT
- `--fp8_scaled`: Better quality fp8 (with `--fp8_base`)
- `--blocks_to_swap N`: Offload blocks to CPU
- `--gradient_checkpointing`
- `--vae_cache_cpu`: CPU caching for VAE

## Inference

Script: `wan_generate_video.py`

### T2V

```bash
python src/musubi_tuner/wan_generate_video.py --fp8 \
  --task t2v-1.3B \
  --video_size 832 480 --video_length 81 --infer_steps 20 \
  --prompt "A cat walks on the grass, realistic style." \
  --save_path path/to/save/dir --output_type both \
  --dit path/to/dit_model \
  --vae path/to/wan_vae.safetensors \
  --t5 path/to/t5_model \
  --seed 1234 --lora_multiplier 1.0 --lora_weight path/to/lora.safetensors
```

### I2V

Add `--image_path path/to/image.jpg` to the T2V command with `--task i2v-14B`.

### Interactive and Batch Modes

- `--interactive`: Enter prompts at command line
- `--from_file prompts.txt`: Batch processing

### Prompt File Inline Parameters

```
A cat walks. --w 832 --h 480 --f 81 --d 42 --s 20
```

- `--w`: Width, `--h`: Height, `--f`: Frames
- `--d`: Seed, `--s`: Steps
- `--n`: Negative prompt, `--l`: Guidance scale
- `--i`: Image path (I2V)
- `--cn`: Control video path (Fun Control)

### Precision Options

- `--fp8`: Basic fp8
- `--fp8_scaled`: Weight optimization (with `--fp8`)
- `--fp8_fast`: RTX 40x0 optimization
- `--fp8_t5`: T5 fp8 mode

### Attention Modes

torch, sdpa, xformers, sageattn, flash2, flash3

### CFG Skip Modes

`--cfg_skip_mode`: early, late, middle, early_late, alternate, none (default)

### Skip Layer Guidance

- `--slg_mode`: `original` (SD3.5) or `uncond` (Wan2GP)
- `--slg_layers`: Block indices to skip
- `--slg_scale`: Guidance scale (default 3.0)
- `--slg_start`, `--slg_end`: Step ratios (0.0-1.0)

## One Frame Inference (Experimental)

Applies FramePack's one-frame concept to Wan2.1. See `references/framepack.md` for full one-frame documentation.

Specify `--one_frame_inference` with `target_index` and `control_index`:

```bash
--output_type latent_images --image_path start.png --control_image_path start.png \
--one_frame_inference control_index=0,target_index=1
```

For one-frame training, specify `--one_frame` in both caching and training scripts.
