# Z-Image

Text-to-image generation with Z-Image. Currently only has a Turbo version (distilled model), making training unstable.

## Model Downloads

- **DiT**: Transformer model from Tongyi-MAI/Z-Image or Comfy-Org/z_image
- **VAE**: Autoencoder model
- **Text Encoder**: Qwen3 model

For split files, specify first file (e.g., `00001-of-00002.safetensors`). Only `*.safetensors` files needed.

### Training-Specific Models (from ostris)

- **De-Turbo model** (`z_image_de_turbo_v1_bf16.safetensors`): Use as DiT for more stable training
- **Training Adapter** (`zimage_turbo_training_adapter_v2.safetensors`): Combine with Turbo via `--base_weights`

## Pre-caching

### Latent Pre-caching

```bash
python src/musubi_tuner/zimage_cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/vae_model
```

Image datasets only. Z-Image does not support control images.

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/zimage_cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --text_encoder path/to/text_encoder \
  --batch_size 16
```

- `--fp8_llm`: Run Qwen3 in fp8 for VRAM savings

## LoRA Training

Script: `zimage_train_network.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/zimage_train_network.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/text_encoder \
  --dataset_config path/to/toml \
  --sdpa --mixed_precision bf16 \
  --timestep_sampling shift --weighting_scheme none --discrete_flow_shift 2.0 \
  --optimizer_type adamw8bit --learning_rate 1e-4 --gradient_checkpointing \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --network_module networks.lora_zimage --network_dim 32 \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-lora
```

### Required Arguments

- `--network_module networks.lora_zimage`
- `--vae` and `--text_encoder`

### Memory Optimization

- `--fp8_base` and `--fp8_scaled`: Reduce DiT memory (use together)
- `--fp8_llm`: Reduce text encoder memory
- `--gradient_checkpointing`
- `--gradient_checkpointing_cpu_offload`
- `--blocks_to_swap N`: Max 28 blocks

### Sampling During Training (De-Turbo/Adapter)

Add negative prompt and CFG:
```
A beautiful landscape. --n bad quality --w 1280 --h 720 --fs 3 --s 20 --d 1234 --l 4
```

`--l` specifies CFG scale (default 4).

## Full Finetuning

Script: `zimage_train.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 src/musubi_tuner/zimage_train.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/text_encoder \
  --dataset_config path/to/toml \
  --sdpa --mixed_precision bf16 --gradient_checkpointing \
  --optimizer_type adafactor --learning_rate 1e-6 --fused_backward_pass \
  --optimizer_args "relative_step=False" "scale_parameter=False" "warmup_init=False" \
  --max_grad_norm 0 --lr_scheduler constant_with_warmup --lr_warmup_steps 10 \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-model
```

### Finetuning Memory Options (by increasing efficiency)

1. `--blocks_to_swap` + `--block_swap_optimizer_patch_params` + compatible optimizer
2. `--blocks_to_swap` + Adafactor + `--fused_backward_pass`
3. `--full_bf16` + Adafactor + `--fused_backward_pass`
4. `--blocks_to_swap` + `--full_bf16` + Adafactor + `--fused_backward_pass`

VRAM: 1 > 2 > 3 > 4 (least). Speed: 2=3 < 4 < 1. Accuracy: 1 > 2 > 3=4.

- `--full_bf16`: bfloat16 weights (saves ~30GB, may reduce accuracy)
- `--fused_backward_pass`: Reduce backward pass VRAM (no gradient accumulation; use `--max_grad_norm 0`)
- `--mem_eff_save`: Reduce RAM during checkpoint saving
- Learning rate: 1e-6 to 1e-5 recommended

## LoRA Conversion to Diffusers

```bash
python src/musubi_tuner/networks/convert_lora.py \
  --input path/to/zimage_lora.safetensors \
  --output path/to/output.safetensors \
  --target other
```

`--target other` for Diffusers/ComfyUI format.

## Inference

Script: `zimage_generate_image.py`

```bash
python src/musubi_tuner/zimage_generate_image.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/text_encoder \
  --prompt "A cat" \
  --image_size 1024 1024 --infer_steps 25 \
  --flow_shift 3.0 --guidance_scale 0.0 \
  --attn_mode torch \
  --save_path path/to/save/dir \
  --seed 1234 --lora_multiplier 1.0 --lora_weight path/to/lora.safetensors
```

### Key Options

- `--flow_shift`: Default 3.0
- `--guidance_scale`: 0.0 for Turbo (no CFG); 4.0 for Base model
- `--fp8`, `--fp8_scaled`: DiT memory reduction
- `--fp8_llm`: Text encoder memory reduction
- `--blocks_to_swap N`: Max 28
- `--save_merged_model`: Merge LoRA and save (skips inference)
