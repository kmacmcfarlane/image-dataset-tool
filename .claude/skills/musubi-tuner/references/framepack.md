# FramePack

Image-to-video generation with progressive frame generation, one-frame inference, and advanced control features (Kisekaeichi, 1f-mc).

## Key Differences from HunyuanVideo

- **I2V only** - no text-to-video support
- Requires an additional Image Encoder (SigLIP)
- Progressive generation = lower VRAM for longer videos
- Uses dedicated `fpack_*.py` scripts

## Model Downloads

### DiT

Choose one:
1. Three `.safetensors` from `lllyasviel/FramePackI2V_HY` (specify first file as `--dit`)
2. Local FramePack installation snapshot
3. Single `FramePackI2V_HY_bf16.safetensors` from `Kijai/HunyuanVideo_comfy`

### VAE

Official HunyuanVideo VAE (recommended), or `vae/diffusion_pytorch_model.safetensors` from hunyuanvideo-community.

### Text Encoder 1 (LLaMA)

`llava_llama3_fp16.safetensors` from Comfy-Org, or 4-part files from hunyuanvideo-community.

### Text Encoder 2 (CLIP)

`clip_l.safetensors` from Comfy-Org.

### Image Encoder (SigLIP)

`sigclip_vision_patch14_384.safetensors` from Comfy-Org or lllyasviel's flux_redux_bfl.

## Pre-caching

Default resolution: 640x640. Dataset must be video-based.

### Latent Pre-caching

```bash
python src/musubi_tuner/fpack_cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/vae_model.safetensors \
  --image_encoder path/to/image_encoder.safetensors \
  --vae_chunk_size 32
```

- Requires `--image_encoder`
- Generates multiple cache files per video with section indices
- `--f1`: FramePack-F1 support
- VRAM options: `--vae_chunk_size`, `--vae_spatial_tile_sample_min_size`, `--vae_tiling`

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/fpack_cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --text_encoder1 path/to/text_encoder1 \
  --text_encoder2 path/to/text_encoder2 \
  --batch_size 16
```

- `--fp8_llm`: LLaMA in fp8 mode

## Training

Script: `fpack_train_network.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/fpack_train_network.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model.safetensors \
  --text_encoder1 path/to/text_encoder1 \
  --text_encoder2 path/to/text_encoder2 \
  --image_encoder path/to/image_encoder.safetensors \
  --dataset_config path/to/toml \
  --sdpa --mixed_precision bf16 \
  --optimizer_type adamw8bit --learning_rate 2e-4 --gradient_checkpointing \
  --timestep_sampling shift --weighting_scheme none --discrete_flow_shift 3.0 \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --network_module networks.lora_framepack --network_dim 32 \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-lora
```

### Required Arguments

- `--vae`, `--text_encoder1`, `--text_encoder2`, `--image_encoder`
- `--network_module networks.lora_framepack`
- `--f1`: For FramePack-F1
- `--latent_window_size` (default 9): Must match caching

### Memory

- Default 640x640 requires ~17GB VRAM
- `--blocks_to_swap N`: Max 36
- `--fp8`, `--fp8_llm`, `--fp8_scaled`
- `--split_attn`: Fix batch size errors

## Inference

Script: `fpack_generate_video.py`

```bash
python src/musubi_tuner/fpack_generate_video.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model.safetensors \
  --text_encoder1 path/to/text_encoder1 \
  --text_encoder2 path/to/text_encoder2 \
  --image_encoder path/to/image_encoder.safetensors \
  --image_path path/to/start_image.jpg \
  --prompt "A cat walks on the grass, realistic style." \
  --video_size 512 768 --video_seconds 5 --fps 30 --infer_steps 25 \
  --attn_mode sdpa --fp8_scaled \
  --vae_chunk_size 32 \
  --save_path path/to/save/dir --output_type both \
  --seed 1234 --lora_multiplier 1.0 --lora_weight path/to/lora.safetensors
```

### Key Options

- `--image_path`: Starting frame (required)
- `--video_seconds` or `--video_sections`: Duration
- `--video_size H W`: Height then width
- `--f1`: FramePack-F1 forward generation
- `--blocks_to_swap N`: Max 38
- `--embedded_cfg_scale` (default 10.0): Distilled guidance
- `--guidance_scale` (default 1.0): CFG (usually 1.0 for base)
- `--bulk_decode`: Decode all frames at once (faster, more VRAM)
- `--latent_window_size` (default 9): Must match training

### Batch and Interactive Modes

```bash
# Batch
python src/musubi_tuner/fpack_generate_video.py --from_file prompts.txt [model args]

# Interactive
python src/musubi_tuner/fpack_generate_video.py --interactive [model args]
```

Prompt file inline parameters:
- `--w`: Width, `--h`: Height, `--f`: Video seconds
- `--d`: Seed, `--s`: Steps, `--g`/`--l`: Guidance
- `--i`: Image path, `--im`: Image mask
- `--ei`: End image, `--ci`: Control image, `--cim`: Control image mask
- `--vs`: Video sections, `--of`: One-frame options

### Advanced Video Control (Experimental)

**End Image Guidance:** `--end_image_path path/to/end.jpg` (incompatible with `--f1`)

**Section Start Images:** `--image_path "start.jpg|mid.jpg"` (pipe-separated per section)

**Section Prompts:** `--prompt "prompt1|prompt2"` (pipe-separated per section)

### RoPE Scaling (Higher Resolution)

- `--rope_scaling_timestep_threshold`: Start ~800
- `--rope_scaling_factor`: Default 0.5 for 2x resolution

## One-Frame Inference (Experimental)

Generate single edited images from a starting image using RoPE manipulation.

### Basic One-Frame

```bash
python src/musubi_tuner/fpack_generate_video.py \
  --video_sections 1 --output_type latent_images \
  --image_path start.png --control_image_path start.png \
  --one_frame_inference default \
  [model args]
```

### One-Frame Options

Comma-separated flags for `--one_frame_inference`:
- `no_2x`: Skip clean latents 2x with zero vectors
- `no_4x`: Skip clean latents 4x with zero vectors
- `no_post`: Skip clean latents post (20% speed boost, potentially unstable)
- `target_index=N`: Generated image index (default: latent_window_size)

### Kisekaeichi Method (Outfit Swapping)

Uses a reference image plus masks for feature merging:

```bash
--image_path start_with_alpha.png \
--control_image_path start_with_alpha.png ref_with_alpha.png \
--control_image_mask_path start_mask.png ref_mask.png \
--one_frame_inference target_index=1,control_index=0;10,no_2x,no_4x,no_post
```

- `target_index`: 1=less change/more noise, 9-13=less noise/larger changes
- `control_index`: Larger than target_index, typically 10
- Masks: 255=referenced, 0=ignored; alpha channels also supported

### 1f-mc (Multi-Control)

Two reference images without masks:

```bash
--image_path start.png \
--control_image_path start.png second.png \
--one_frame_inference target_index=9,control_index=0;1,no_2x,no_4x
```

Intended for use with LoRA training.

## One-Frame Training

Requires image datasets. Specify `--one_frame` in both caching and training scripts.

Dataset config: `control_directory` has starting images; `image_directory` has target (changed) images with matching filenames.

JSONL format: `{"image_path": "/path/to/target.jpg", "control_path": "/path/to/source.jpg", "caption": "description"}`

Cache with: `--one_frame` (plus optional `--one_frame_no_2x`, `--one_frame_no_4x`)
