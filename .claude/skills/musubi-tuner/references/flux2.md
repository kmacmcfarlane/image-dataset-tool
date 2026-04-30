# FLUX.2

Image generation and editing with FLUX.2 [dev], FLUX.2 [klein] 4B/9B, and base variants.

## Model Variants

| Variant | `--model_version` | Guidance Scale | Steps | Notes |
|---------|-------------------|---------------|-------|-------|
| FLUX.2 dev | `dev` | 4.0 | 50 | Distilled, for inference |
| FLUX.2 klein-4b | `klein-4b` | 1.0 | 4 | Distilled, for inference |
| FLUX.2 klein-base-4b | `klein-base-4b` | 4.0 | 50 | **Use for training** |
| FLUX.2 klein-9b | `klein-9b` | 1.0 | 4 | Distilled, for inference |
| FLUX.2 klein-base-9b | `klein-base-9b` | 4.0 | 50 | **Use for training** |

Use **base** 4B or 9B for training; dev and klein 4B/9B are distilled models for inference only.

## Model Downloads

**FLUX.2 [dev]:**
- DiT: `flux2-dev.safetensors` from black-forest-labs/FLUX.2-dev
- AE: `ae.safetensors` from black-forest-labs/FLUX.2-dev
- Text Encoder (Mistral 3): Download split files, specify first file like `00001-of-00010.safetensors`

**FLUX.2 [klein] 4B / base 4B:**
- DiT: `flux2-klein-4b.safetensors` or `flux2-klein-base-4b.safetensors`
- AE: `ae.safetensors` from FLUX.2-dev repo
- Text Encoder (Qwen3 4B): First split file from FLUX.2-klein-4B repo

**FLUX.2 [klein] 9B / base 9B:**
- DiT: `flux2-klein-9b.safetensors` or `flux2-klein-base-9b.safetensors`
- AE: `ae.safetensors` from FLUX.2-dev repo
- Text Encoder (Qwen3 8B): First split file from FLUX.2-klein-9B repo

## Pre-caching

### Latent Pre-caching

```bash
python src/musubi_tuner/flux_2_cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/ae_model \
  --model_version dev
```

- Use `--vae` (not `--ae`)
- Dataset must be image-based
- `control_images` in dataset config serves as reference images
- `--vae_dtype`: supports float32 or bfloat16

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/flux_2_cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --text_encoder path/to/text_encoder \
  --batch_size 16 \
  --model_version dev
```

- `--fp8_text_encoder`: fp8 mode for VRAM savings
- Adjust `--batch_size` per available VRAM

## Training

Script: `flux_2_train_network.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/flux_2_train_network.py \
  --model_version dev \
  --dit path/to/dit_model \
  --vae path/to/ae_model \
  --text_encoder path/to/text_encoder \
  --dataset_config path/to/toml \
  --sdpa --mixed_precision bf16 \
  --timestep_sampling flux2_shift --weighting_scheme none \
  --optimizer_type adamw8bit --learning_rate 1e-4 --gradient_checkpointing \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --network_module networks.lora_flux_2 --network_dim 32 \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-lora
```

### Required Arguments

- `--model_version`: dev, klein-4b, klein-base-4b, klein-9b, or klein-base-9b
- `--vae` (not `--ae`)
- `--text_encoder`
- `--network_module networks.lora_flux_2`
- `--timestep_sampling flux2_shift`
- `--mixed_precision bf16`

### Memory Options

- `--fp8_base`, `--fp8_scaled`, `--fp8_text_encoder`
- `--gradient_checkpointing`
- `--blocks_to_swap N`: Max with fp8: dev=29, klein-4b=13, klein-9b=16

## Inference

Script: `flux_2_generate_image.py`

```bash
python src/musubi_tuner/flux_2_generate_image.py \
  --model_version dev \
  --dit path/to/dit_model \
  --vae path/to/ae_model \
  --text_encoder path/to/text_encoder \
  --control_image_path path/to/control_image.jpg \
  --prompt "A cat" \
  --image_size 1024 1024 --infer_steps 50 \
  --fp8_scaled \
  --save_path path/to/save/dir --output_type images \
  --seed 1234 --lora_multiplier 1.0 --lora_weight path/to/lora.safetensors
```

### Inference Options

- `--control_image_path`: Reference image for editing
- `--no_resize_control`: Skip resizing control image (experimental)
- `--fp8_scaled`: DiT memory reduction
- `--fp8_text_encoder`: Text encoder memory reduction
- `--embedded_cfg_scale` (default 2.5): Distilled guidance scale
- `--lora_weight`, `--lora_multiplier`: LoRA loading
- `--include_patterns`, `--exclude_patterns`: LoRA pattern filtering
- `--save_merged_model`: Save merged DiT after LoRA (skips inference)
