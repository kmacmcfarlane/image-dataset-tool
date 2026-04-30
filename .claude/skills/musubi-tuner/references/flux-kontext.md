# FLUX.1 Kontext

Image generation with reference image input using FLUX.1 Kontext [dev].

## Model Downloads

- **DiT**: `flux1-kontext-dev.safetensors` from black-forest-labs/FLUX.1-kontext (subfolder weights are Diffusers format and cannot be used)
- **AE**: `ae.safetensors` from same repo
- **Text Encoder 1 (T5-XXL)**: `t5xxl_fp16.safetensors` from ComfyUI FLUX Text Encoders repo
- **Text Encoder 2 (CLIP-L)**: `clip_l.safetensors` from same repo

## Pre-caching

### Latent Pre-caching

```bash
python src/musubi_tuner/flux_kontext_cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/ae_model
```

- Use `--vae` (not `--ae`)
- Dataset must be image-based
- `control_images` in dataset config is used as the reference image

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/flux_kontext_cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --text_encoder1 path/to/t5xxl \
  --text_encoder2 path/to/clip_l \
  --batch_size 16
```

- Requires both `--text_encoder1` (T5) and `--text_encoder2` (CLIP)
- `--fp8_t5`: Run T5 in fp8 for VRAM savings

## Training

Script: `flux_kontext_train_network.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/flux_kontext_train_network.py \
  --dit path/to/dit_model \
  --vae path/to/ae_model \
  --text_encoder1 path/to/t5xxl \
  --text_encoder2 path/to/clip_l \
  --dataset_config path/to/toml \
  --sdpa --mixed_precision bf16 \
  --timestep_sampling flux_shift --weighting_scheme none \
  --optimizer_type adamw8bit --learning_rate 1e-4 --gradient_checkpointing \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --network_module networks.lora_flux --network_dim 32 \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-lora
```

### Required Arguments

- `--vae` (not `--ae`)
- `--text_encoder1` and `--text_encoder2`
- `--network_module networks.lora_flux`
- `--timestep_sampling flux_shift`
- `--mixed_precision bf16`

### Memory Options

- `--fp8` / `--fp8_scaled` (recommended): fp8 for DiT
- `--fp8_t5`: fp8 for Text Encoder 1
- `--gradient_checkpointing`
- `--gradient_checkpointing_cpu_offload`

## Inference

Script: `flux_kontext_generate_image.py`

```bash
python src/musubi_tuner/flux_kontext_generate_image.py \
  --dit path/to/dit_model \
  --vae path/to/ae_model \
  --text_encoder1 path/to/t5xxl \
  --text_encoder2 path/to/clip_l \
  --control_image_path path/to/control_image.jpg \
  --prompt "A cat" \
  --image_size 1024 1024 --infer_steps 25 \
  --attn_mode sdpa --fp8_scaled \
  --save_path path/to/save/dir --output_type images \
  --seed 1234 --lora_multiplier 1.0 --lora_weight path/to/lora.safetensors
```

### Inference Options

- `--control_image_path`: Reference image (required)
- `--image_size H W`: Height then width
- `--no_resize_control`: Skip resizing to recommended resolution (experimental)
- `--fp8_scaled`: DiT memory reduction (better than `--fp8` alone)
- `--fp8_t5`: Text Encoder 1 memory reduction
- `--embedded_cfg_scale` (default 2.5): Distilled guidance scale
- `--lora_weight`, `--lora_multiplier`: LoRA loading
- `--include_patterns`, `--exclude_patterns`: LoRA pattern filtering
- `--lycoris`: LyCORIS support
- `--save_merged_model`: Save merged DiT (skips inference)
